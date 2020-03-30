package filer

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
)

const (
	// API_NS_SVMS is the API namespace for OnTAP SVM items.
	API_NS_SVMS string = "/svm/svms"
	// API_NS_JOBS is the API namespace for OnTAP cluster job items.
	API_NS_JOBS string = "/cluster/jobs"
	// API_NS_VOLUMES is the API namespace for OnTAP volume items.
	API_NS_VOLUMES string = "/storage/volumes"
	// API_NS_AGGREGATES is the API namespace for OnTAP aggregate items.
	API_NS_AGGREGATES string = "/storage/aggregates"
	// API_NS_QTREES is the API namespace for OnTAP qtree items.
	API_NS_QTREES string = "/storage/qtrees"
	// API_NS_QUOTA_RULES is the API namespace for OnTAP quota rule items.
	API_NS_QUOTA_RULES string = "/storage/quota/rules"
)

// NetApp implements Filer interface for NetApp OnTAP cluster.
type NetApp struct {
	// APIServerURL is the server URL of the OnTAP APIs.
	APIServerURL string
	// APIUsername is the username for the basic authentication of the OnTAP API.
	APIUsername string
	// APIPassword is the password for the basic authentication of the OnTAP API.
	APIPassword string
	// ProjectMode specifies how the project space is allocated. Valid modes are
	// "volume" and "qtree".
	ProjectMode string
	// Vserver specifies the name of OnTAP SVM on which the filer APIs will perform.
	Vserver string
	// ProjectUID specifies the system UID of user `project`
	ProjectUID int
	// ProjectGID specifies the system GID of group `project_g`
	ProjectGID int
	// ProjectRoot specifies the top-level NAS path in which projects are located.
	ProjectRoot string
}

// volName converts project identifier to the OnTAP volume name.
//
// e.g. 3010000.01 -> project_3010000_01
func (filer NetApp) volName(projectID string) string {
	return strings.Join([]string{
		"project",
		strings.ReplaceAll(projectID, ".", "_"),
	}, "_")
}

// CreateProject provisions a project space on the filer with the given quota.
func (filer NetApp) CreateProject(projectID string, quotaGiB int) error {

	switch filer.ProjectMode {
	case "volume":
		// check if volume with the same name doee not exist.
		qry := url.Values{}
		qry.Set("name", filer.volName(projectID))
		records, err := filer.GetRecordsByQuery(qry, API_NS_VOLUMES)
		if err != nil {
			return fmt.Errorf("fail to check volume %s: %s", projectID, err)
		}
		if len(records) != 0 {
			return fmt.Errorf("project volume already exists: %s", projectID)
		}

		// determine which aggregate should be used for creating the new volume.
		quota := int64(quotaGiB << 30)
		svm := SVM{}
		if err := filer.GetObjectByName(filer.Vserver, API_NS_SVMS, &svm); err != nil {
			return fmt.Errorf("fail to get SVM %s: %s", filer.Vserver, err)
		}
		avail := int64(0)

		var theAggr *Aggregate
		for _, record := range svm.Aggregates {
			aggr := Aggregate{}
			href := strings.Join([]string{
				"/api",
				API_NS_AGGREGATES,
				record.UUID,
			}, "/")
			if err := filer.GetObjectByHref(href, &aggr); err != nil {
				log.Errorf("ignore aggregate %s: %s", record.Name, err)
			}
			if aggr.State == "online" && aggr.Space.BlockStorage.Available > avail && aggr.Space.BlockStorage.Available > quota {
				theAggr = &aggr
			}
		}

		if theAggr == nil {
			return fmt.Errorf("cannot find aggregate for creating volume")
		}
		log.Debugf("selected aggreate for project volume: %+v", *theAggr)

		// create project volume with given quota.
		vol := Volume{
			Name: filer.volName(projectID),
			Aggregates: []Record{
				Record{Name: theAggr.Name},
			},
			Size:  quota,
			Svm:   Record{Name: filer.Vserver},
			State: "online",
			Style: "flexvol",
			Type:  "rw",
			Nas: Nas{
				UID:             filer.ProjectUID,
				GID:             filer.ProjectGID,
				Path:            filepath.Join(filer.ProjectRoot, projectID),
				SecurityStyle:   "unix",
				UnixPermissions: "0750",
				ExportPolicy:    ExportPolicy{Name: "dccn-projects"},
			},
			QoS: &QoS{
				Policy: QoSPolicy{MaxIOPS: 6000},
			},
			Autosize: &Autosize{Mode: "off"},
		}

		// blocking operation to create the volume.
		if err := filer.createObject(&vol, API_NS_VOLUMES); err != nil {
			return err
		}

	case "qtree":

	default:
		return fmt.Errorf("unsupported project mode: %s", filer.ProjectMode)
	}

	return nil
}

func (filer NetApp) CreateHome(username, groupname string, quotaGiB int) error {

	// check if volume "groupname" exists.

	// check if qtree with "username" already exists.

	// create qtree within the volume.

	// wait until the qtree creation to finish.

	// make sure quota rule doesn't exist in advance.

	// switch off volume quota
	// otherwise we cannot create new rule; perhaps it is because we don't have default rule on the volume.

	// create quota rule for the newly created qtree.

	// apply the quota rule (turn volume quota on/off)

	return nil
}

func (filer NetApp) SetProjectQuota(projectID string, quotaGiB int) error {
	switch filer.ProjectMode {
	case "volume":
		// check if volume with the same name already exists.

		// resize the volume to the given quota.

	case "qtree":

	default:
		return fmt.Errorf("unsupported project mode: %s", filer.ProjectMode)
	}

	return nil
}

func (filer NetApp) SetHomeQuota(username, groupname string, quotaGiB int) error {

	// check if the qtree "username" already exists under volume "groupname"

	// update corresponding quota rule for the qtree

	// turn volume quota off and on to apply the new rule.

	return nil
}

// GetVolume gets the volume with the given name.
func (filer NetApp) GetVolume(name string) (*Volume, error) {

	volume := Volume{}

	if err := filer.GetObjectByName(name, API_NS_VOLUMES, &volume); err != nil {
		return nil, err
	}

	return &volume, nil
}

// GetObjectByName retrives the named object from the given API namespace.
func (filer NetApp) GetObjectByName(name, nsAPI string, object interface{}) error {

	query := url.Values{}
	query.Set("name", name)

	records, err := filer.GetRecordsByQuery(query, nsAPI)
	if err != nil {
		return err
	}

	if len(records) != 1 {
		return fmt.Errorf("more than 1 object found: %d", len(records))
	}

	if err := filer.GetObjectByHref(records[0].Link.Self.Href, object); err != nil {
		return err
	}

	return nil
}

// GetRecordsByQuery retrives the object from the given API namespace using a specific URL query.
func (filer NetApp) GetRecordsByQuery(query url.Values, nsAPI string) ([]Record, error) {

	records := make([]Record, 0)

	c := newHTTPSClient()

	href := strings.Join([]string{filer.APIServerURL, "api", nsAPI}, "/")

	// create request
	req, err := http.NewRequest("GET", href, nil)
	if err != nil {
		return records, err
	}

	req.URL.RawQuery = query.Encode()

	// set request header for basic authentication
	req.SetBasicAuth(filer.APIUsername, filer.APIPassword)
	// NOTE: adding "Accept: application/json" to header can causes the API server
	//       to not returning "_links" attribute containing API href to the object.
	//       Therefore, it is not set here.
	//req.Header.Set("accept", "application/json")

	res, err := c.Do(req)
	if err != nil {
		return records, err
	}

	// expect status to be 200 (OK)
	if res.StatusCode != 200 {
		return records, fmt.Errorf("response not ok: %s (%d)", res.Status, res.StatusCode)
	}

	// read response body
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return records, err
	}

	log.Debugf("%s", string(httpBodyBytes))

	// unmarshal response body to object structure
	rec := Records{}
	if err := json.Unmarshal(httpBodyBytes, &rec); err != nil {
		return records, err
	}

	return rec.Records, nil
}

// GetObjectByHref retrives the object from the given API namespace using a specific URL query.
func (filer NetApp) GetObjectByHref(href string, object interface{}) error {

	c := newHTTPSClient()

	// create request
	req, err := http.NewRequest("GET", strings.Join([]string{filer.APIServerURL, href}, "/"), nil)
	if err != nil {
		return err
	}

	// set request header for basic authentication
	req.SetBasicAuth(filer.APIUsername, filer.APIPassword)
	// NOTE: adding "Accept: application/json" to header can causes the API server
	//       to not returning "_links" attribute containing API href to the object.
	//       Therefore, it is not set here.
	//req.Header.Set("accept", "application/json")

	res, err := c.Do(req)

	// expect status to be 200 (OK)
	if res.StatusCode != 200 {
		return fmt.Errorf("response not ok: %s (%d)", res.Status, res.StatusCode)
	}

	// read response body
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	log.Debugf("%s", string(httpBodyBytes))

	// unmarshal response body to object structure
	if err := json.Unmarshal(httpBodyBytes, object); err != nil {
		return err
	}

	return nil
}

// createObject creates given object under the specified API namespace.
func (filer NetApp) createObject(object interface{}, nsAPI string) error {
	c := newHTTPSClient()

	href := strings.Join([]string{filer.APIServerURL, "api", nsAPI}, "/")

	data, err := json.Marshal(object)

	if err != nil {
		return fmt.Errorf("fail to convert to json data: %+v, %s", object, err)
	}

	log.Debugf("object creation input: %s", string(data))

	// create request
	req, err := http.NewRequest("POST", href, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	// set request header for basic authentication
	req.SetBasicAuth(filer.APIUsername, filer.APIPassword)
	req.Header.Set("content-type", "application/json")

	res, err := c.Do(req)

	// expect status to be 202 (Accepted)
	if res.StatusCode != 202 {
		// try to get the error code returned as the body
		var apiErr APIError
		if httpBodyBytes, err := ioutil.ReadAll(res.Body); err == nil {
			json.Unmarshal(httpBodyBytes, &apiErr)
		}
		return fmt.Errorf("response not ok: %s (%d), error: %+v", res.Status, res.StatusCode, apiErr)
	}

	// read response body as accepted job
	httpBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("cannot read response body: %s", err)
	}

	log.Debugf("%s", string(httpBodyBytes))

	job := APIJob{}
	// unmarshal response body to object structure
	if err := json.Unmarshal(httpBodyBytes, &job); err != nil {
		return fmt.Errorf("cannot get job id: %s", err)
	}

	log.Debugf("%+v", job)

	if err := filer.waitJob(&job); err != nil {
		return err
	}

	if job.Job.State != "success" {
		return fmt.Errorf("API job failed: %s", job.Job.Message)
	}

	return nil
}

// waitJob polls the status of the api job unti it if finished; and reports the job's final state.
func (filer NetApp) waitJob(job *APIJob) error {

	var err error

	href := job.Job.Link.Self.Href

waitLoop:
	for {
		if e := filer.GetObjectByHref(href, &(job.Job)); err != nil {
			err = fmt.Errorf("cannot poll job %s: %s", job.Job.UUID, e)
			break
		}

		log.Debugf("job status: %s", job.Job.State)

		switch job.Job.State {
		case "success":
			break waitLoop
		case "failure":
			break waitLoop
		default:
			time.Sleep(3 * time.Second)
			continue waitLoop
		}
	}

	return err
}

// internal utility functions
func newHTTPSClient() (client *http.Client) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // FIXIT: don't ignore the bad server certificate.
	}

	client = &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}

	return
}

// APIJob of the API request.
type APIJob struct {
	Job Job `json:"job"`
}

// Job detail of the API request.
type Job struct {
	Link    *Link  `json:"_links"`
	UUID    string `json:"uuid"`
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
}

// APIError of the API request.
type APIError struct {
	Error struct {
		Target    string `json:"target"`
		Arguments struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"arguments"`
	} `json:"error"`
}

// Records of the items within an API namespace.
type Records struct {
	NumberOfRecords int      `json:"num_records"`
	Records         []Record `json:"records"`
}

// Record of an item within an API namespace.
type Record struct {
	UUID string `json:"uuid,omitempty"`
	Name string `json:"name,omitempty"`
	Link *Link  `json:"_links,omitempty"`
}

// Link of an item for retriving the detail.
type Link struct {
	Self struct {
		Href string `json:"href"`
	} `json:"self"`
}

// Volume of OnTAP.
type Volume struct {
	UUID       string    `json:"uuid,omitempty"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	State      string    `json:"state"`
	Size       int64     `json:"size"`
	Style      string    `json:"style"`
	Space      *Space    `json:"space,omitempty"`
	Svm        Record    `json:"svm"`
	Aggregates []Record  `json:"aggregates"`
	Nas        Nas       `json:"nas"`
	QoS        *QoS      `json:"qos,omitempty"`
	Autosize   *Autosize `json:"autosize,omitempty"`
	Link       *Link     `json:"_links,omitempty"`
}

// QoS contains a Qolity-of-Service policy.
type QoS struct {
	Policy QoSPolicy `json:"policy"`
}

// QoSPolicy defines the data structure of the QoS policy.
type QoSPolicy struct {
	MaxIOPS int    `json:"max_throughput_iops,omitempty"`
	MaxMBPS int    `json:"max_throughput_mbps,omitempty"`
	UUID    string `json:"uuid,omitempty"`
	Name    string `json:"name,omitempty"`
}

// Autosize defines the volume autosizing mode
type Autosize struct {
	Mode string `json:"mode"`
}

// Nas related attribute of OnTAP.
type Nas struct {
	Path            string       `json:"path,omitempty"`
	UID             int          `json:"uid,omitempty"`
	GID             int          `json:"gid,omitempty"`
	SecurityStyle   string       `json:"security_style,omitempty"`
	UnixPermissions string       `json:"unix_permissions,omitempty"`
	ExportPolicy    ExportPolicy `json:"export_policy,omitempty"`
}

// ExportPolicy
type ExportPolicy struct {
	Name string `json:"name"`
}

// Space information of a OnTAP volume.
type Space struct {
	Size      int64 `json:"size"`
	Available int64 `json:"available"`
	Used      int64 `json:"used"`
}

// SVM of OnTAP
type SVM struct {
	UUID       string   `json:"uuid"`
	Name       string   `json:"name"`
	State      string   `json:"state"`
	Aggregates []Record `json:"aggregates"`
}

// Aggregate of OnTAP
type Aggregate struct {
	UUID  string `json:"uuid"`
	Name  string `json:"name"`
	State string `json:"state"`
	Space struct {
		BlockStorage Space `json:"block_storage"`
	} `json:"space"`
}
