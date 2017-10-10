package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/ogier/pflag"
	"github.com/tidwall/gjson"
)

var listenPort uint
var consulAddressAndPort string

func init() {
	pflag.UintVarP(&listenPort, "port", "p", 31998, "siok listening port")
	pflag.StringVarP(&consulAddressAndPort, "agent", "a", "127.0.0.1:8500", "Consul Agent IP:port")
	pflag.Parse()
}

// getChecks creates an aggregate check output related
func getChecks(serviceID string) []Check {

	var checks []Check

	res, err := http.Get(fmt.Sprintf("http://%v/v1/agent/checks", consulAddressAndPort))
	if err != nil {
		// failure to connect to local agent should return critical
		checks = append(checks, Check{Output: err.Error(), Notes: "Consul Agent unavailable", Status: "critical"})
		return checks
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		// failure to parse the response from the local agent should return critical
		checks = append(checks, Check{Output: err.Error(), Notes: "Failed to parse the response from the Consul Agent", Status: "critical"})
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
		checks = append(checks, Check{ServiceID: serviceID, Output: "No such service or no check associated with it", Status: "critical"})

	}
	return checks
}

// parseChecks returns the status, in the following order: critical > warning > passing
func parseChecks(checks []Check) string {
	var passing, warning, critical bool

	for _, check := range checks {
		switch check.Status {
		case "passing":
			passing = true
		case "warning":
			warning = true
		case "critical":
			critical = true
		default:
			passing = true
		}
	}

	switch {
	case critical:
		return "critical"
	case warning:
		return "warning"
	case passing:
		return "passing"
	default:
		return "passing"
	}
}

func getServiceHealth(c *gin.Context) {

	var service Service
	var warn Warn
	c.BindQuery(&service)
	c.BindQuery(&warn)
	warnEnabled := parseBoolValue(warn.warn)

	checks := getChecks(service.ID)

	aggregatedStatus := parseChecks(checks)

	// converting checks to an interface to be used by c.JSON
	var interfaceSlice = make([]interface{}, len(checks))
	for i, check := range checks {
		interfaceSlice[i] = check
	}

	switch aggregatedStatus {
	case "passing":
		c.JSON(200, interfaceSlice)
	case "warning":
		if warnEnabled {
			c.Header("Warning", "Some Consul checks failed, please investigate")
			c.JSON(200, interfaceSlice)
		} else {
			c.JSON(503, interfaceSlice)
		}
	case "critical":
		c.JSON(503, interfaceSlice)
	}
}

func parseBoolValue(val string) bool {
	if val == "true" {
		return true
	}
	return false
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	route := gin.Default()
	route.GET("/health", getServiceHealth)
	log.Printf("Running siok on port %v...", listenPort)
	err := route.Run(fmt.Sprintf(":%v", listenPort))
	if err != nil {
		log.Printf("siok failed to start: %v", err.Error())
		os.Exit(1)
	}
}

// Service defines the data parsed from URL's querystring
type Service struct {
	ID string `form:"service"`
}

// Warn defines the data parsed from URL's querystring
type Warn struct {
	warn string `form:"warn"`
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
