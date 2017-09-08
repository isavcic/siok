package main

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ogier/pflag"
	"github.com/tidwall/gjson"
)

var listenPort uint

func init() {
	pflag.UintVarP(&listenPort, "port", "p", 31998, "API listening port")
	pflag.Parse()
}

// parseServiceListing creates an aggregate check output related
func parseServiceListing(serviceID string) []Check {

	var checks []Check

	res, err := http.Get("http://127.0.0.1:8500/v1/agent/checks")
	if err != nil {
		// failure to connect to local agent should return critical
		checks = append(checks, Check{Output: err.Error(), Status: "critical"})
		return checks
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		// failure to parse the response from the local agent should return critical
		checks = append(checks, Check{Output: err.Error(), Status: "critical"})
		return checks
	}

	serviceExists := false
	result := gjson.ParseBytes(body)
	result.ForEach(func(_, value gjson.Result) bool {
		jsonServiceID := value.Get("ServiceID").String()

		if jsonServiceID == serviceID {
			serviceExists = true
		}

		// Reminder: if there is no service ID, that means the check/maint is node related
		if (jsonServiceID == serviceID) || (jsonServiceID == "") {
			var check Check

			check.ServiceID = jsonServiceID
			check.Name = value.Get("Name").String()
			check.Notes = value.Get("Notes").String()
			check.Output = value.Get("Output").String()
			check.Status = value.Get("Status").String()
			check.CheckID = value.Get("CheckID").String()

			checks = append(checks, check)
		}
		return true
	})

	if !serviceExists {
		checks = append(checks, Check{ServiceID: serviceID, Output: "No such service", Status: "critical"})

	}
	return checks
}

func getServiceHealth(c *gin.Context) {

	var service Service
	c.BindQuery(&service)

	checks := parseServiceListing(service.ID)

	// status is an "internal" variable (not returned to the client by the API)
	// which, when not "passing", will cause us to return HTTP 503, so there's currently no need
	// to be extra pedantic here, ie. we only care if it's not "passing"
	status := "passing"
	for _, check := range checks {
		if check.Status != "passing" {
			status = check.Status
			break
		}
	}

	// converting checks to an interface to be used by c.JSON
	var interfaceSlice = make([]interface{}, len(checks))
	for i, check := range checks {
		interfaceSlice[i] = check
	}

	if string(status) != "passing" {
		c.JSON(503, interfaceSlice)
	} else {
		c.JSON(200, interfaceSlice)
	}
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	route := gin.Default()
	route.GET("/health", getServiceHealth)
	route.Run(fmt.Sprintf(":%v", listenPort))
}

// Service defines the data parsed from URL's querystring
type Service struct {
	ID string `form:"service"`
}

// Check defines the check data to be returned via the API's JSON response
type Check struct {
	CheckID   string `json:"CheckID"`
	Name      string `json:"Name"`
	Notes     string `json:"Notes"`
	Output    string `json:"Output"`
	ServiceID string `json:"ServiceID"`
	Status    string `json:"Status"`
}
