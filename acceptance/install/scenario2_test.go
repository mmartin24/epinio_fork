package install_test

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/route53"
	"github.com/epinio/epinio/acceptance/testenv"
)

// This test uses AWS route53 to update the system domain's records
var _ = Describe("<Scenario2> GKE, Letsencrypt-staging, Zero instance", func() {
	var (
		flags        []string
		epinioHelper epinio.Epinio
		appName      = catalog.NewAppName()
		loadbalancer string
		domain       string
		zoneID       string
		instancesNum string
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		domain = os.Getenv("EPINIO_SYSTEM_DOMAIN")
		Expect(domain).ToNot(BeEmpty())

		zoneID = os.Getenv("AWS_ZONE_ID")
		Expect(zoneID).ToNot(BeEmpty())

		instancesNum = "0"

		flags = []string{
			"--set", "global.domain=" + domain,
			"--set", "global.tlsIssuer=letsencrypt-staging",
		}
	})

	AfterEach(func() {
		out, err := epinioHelper.Uninstall()
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("Installs with letsencrypt-staging cert, custom domain and pushes an app with 0 instances", func() {
		By("Creating letsencrypt issuer", func() {
			// Create certificate secret and cluster_issuer
			out, err := proc.RunW("kubectl", "apply", "-f",
				testenv.TestAssetPath("letsencrypt-staging.yaml"))
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Checking LoadBalancer IP", func() {
			// Ensure that Traefik LB is not in Pending state anymore, could take time
			Eventually(func() string {
				out, err := proc.RunW("kubectl", "get", "svc", "-n", "traefik", "traefik", "--no-headers")
				Expect(err).NotTo(HaveOccurred(), out)
				return out
			}, "4m", "2s").ShouldNot(ContainSubstring("<pending>"))

			out, err := proc.RunW("kubectl", "get", "service", "-n", "traefik", "traefik", "-o", "json")
			Expect(err).NotTo(HaveOccurred(), out)

			// Check that an IP address for LB is configured
			status := &testenv.LoadBalancerHostname{}
			err = json.Unmarshal([]byte(out), status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status.Status.LoadBalancer.Ingress).To(HaveLen(1))
			loadbalancer = status.Status.LoadBalancer.Ingress[0].IP
			Expect(loadbalancer).ToNot(BeEmpty())
		})

		By("Updating DNS Entries", func() {
			change := route53.A(domain, loadbalancer, "UPSERT")
			out, err := route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)

			change = route53.A("*."+domain, loadbalancer, "UPSERT")
			out, err = route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)
		})

		// Check that DNS entry is correctly propagated
		By("Checking that DNS entry is correctly propagated", func() {
			Eventually(func() string {
				out, err := route53.TestDnsAnswer(zoneID, domain, "A")
				Expect(err).NotTo(HaveOccurred(), out)

				answer := &route53.DNSAnswer{}
				err = json.Unmarshal([]byte(out), answer)
				Expect(err).NotTo(HaveOccurred())
				if len(answer.RecordData) == 0 {
					return ""
				}
				return answer.RecordData[0]
			}, "5m", "2s").Should(Equal(loadbalancer))
		})

		// Workaround to (try to!) ensure that the DNS is really propagated!
		time.Sleep(3 * time.Minute)

		By("Installing Epinio", func() {
			out, err := epinioHelper.Install(flags...)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("STATUS: deployed"))

			out, err = testenv.PatchEpinio()
			Expect(err).ToNot(HaveOccurred(), out)
		})

		By("Updating Epinio settings", func() {
			out, err := epinioHelper.Run("settings", "update")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Ok"))
		})

		By("Checking Epinio info command", func() {
			Eventually(func() string {
				out, _ := epinioHelper.Run("info")
				return out
			}, "2m", "2s").Should(ContainSubstring("Epinio Server Version:"))
		})

		By("Pushing an app with zero instances", func() {
			out, err := epinioHelper.Run("push",
				"--name", appName,
				"--path", testenv.AssetPath("sample-app"),
				"--instances", instancesNum)
			Expect(err).ToNot(HaveOccurred(), out)

			Eventually(func() string {
				out, err := proc.Kubectl("get", "deployments",
					"-l", fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s", appName, testenv.DefaultWorkspace),
					"--namespace", testenv.DefaultWorkspace,
					"-o", "jsonpath={.items[].spec.replicas}")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "30s", "1s").Should(Equal("0"))

			// Verify cluster_issuer is used
			out, err = proc.RunW("kubectl", "get", "certificate",
				"-n", testenv.DefaultWorkspace,
				"--selector", "app.kubernetes.io/name="+appName,
				"-o", "jsonpath='{.items[*].spec.issuerRef.name}'")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal("'letsencrypt-staging'"))
		})

		By("Cleaning DNS Entries", func() {
			change := route53.A(domain, loadbalancer, "DELETE")
			out, err := route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)

			change = route53.A("*."+domain, loadbalancer, "DELETE")
			out, err = route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)
		})
	})
})
