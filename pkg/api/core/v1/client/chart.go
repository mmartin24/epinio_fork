// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// ChartList returns a list of all known application charts
func (c *Client) ChartList() ([]models.AppChart, error) {
	response := []models.AppChart{}
	endpoint := api.Routes.Path("ChartList")

	return Get(c, endpoint, response)
}

// ChartShow returns a named application chart
func (c *Client) ChartShow(name string) (models.AppChart, error) {
	response := models.AppChart{}
	endpoint := api.Routes.Path("ChartShow", name)

	return Get(c, endpoint, response)
}

// ChartMatch returns all application charts whose name matches the prefix
func (c *Client) ChartMatch(prefix string) (models.ChartMatchResponse, error) {
	response := models.ChartMatchResponse{}
	endpoint := api.Routes.Path("ChartMatch", prefix)

	return Get(c, endpoint, response)
}
