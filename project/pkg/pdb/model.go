package pdb

import (
	"fmt"
	"regexp"
	"time"
)

// Member defines the data structure of a pending role setting for a project member.
type Member struct {
	UserID string `json:"userID"`
	Role   string `json:"role"`
	// The `Timestamp` attribute is used for registering a timestamp concerning the
	// time the member role request is created. Currently, this is needed for PDBv1
	// to operate when cleaning up successfully performed pending roles in the SQL
	// database. This attribute is ignored for JSON (un)marshal.
	Timestamp time.Time `json:"-"`
}

// Storage defines the data structure for the storage resource of a project.
type Storage struct {
	QuotaGb int    `json:"quotaGb"`
	System  string `json:"system"`
}

// StorageInfo defines the data structure for the storage resource information of a project,
// including the actual storage usage.
type StorageInfo struct {
	QuotaGb int    `json:"quotaGb"`
	UsageMb int    `json:"usageMb"`
	System  string `json:"system"`
}

// DataProjectProvision defines the data structure for sending project provision
// request to the filer-gateway.
type DataProjectProvision struct {
	ProjectID string   `json:"projectID"`
	Members   []Member `json:"members"`
	Storage   Storage  `json:"storage"`
}

// DataProjectUpdate defines the data structure for sending project update request
// to the filer-gateway.
type DataProjectUpdate struct {
	Members []Member `json:"members"`
	Storage Storage  `json:"storage"`
}

// DataProjectInfo defines the data structure for received project storage information
// returned from the filer-gateway.
type DataProjectInfo struct {
	ProjectID string      `json:"projectID"`
	Members   []Member    `json:"members"`
	Storage   StorageInfo `json:"storage"`
}

// Project defines the data structure of a project in the project database.
type Project struct {
	ID     string        `json:"projectID"`
	Name   string        `json:"projectName"`
	Owner  string        `json:"owner"`
	Status ProjectStatus `json:"status"`
}

// ProjectStatus defines PDB project status.
type ProjectStatus int

const (
	// ProjectStatusUnknown refers to unexpected/unknown project status in PDB.
	ProjectStatusUnknown ProjectStatus = iota - 1
	// ProjectStatusActive refers to active project.
	ProjectStatusActive
	// ProjectStatusInactive refers to inactive project.
	ProjectStatusInactive
)

// User defines the data structure of a user in the project database.
type User struct {
	ID         string       `json:"userID"`
	Firstname  string       `json:"firstName"`
	Middlename string       `json:"middleName"`
	Lastname   string       `json:"lastName"`
	Email      string       `json:"email"`
	Status     UserStatus   `json:"status"`
	Function   UserFunction `json:"function"`
}

// UserFunction defines PDB user function.
// TODO: refine the fine-grained user functions.
type UserFunction int

const (
	// UserFunctionOther for other functions not indicated below.
	UserFunctionOther UserFunction = iota - 1
	// UserFunctionPrincipalInvestigator for users with the principle investigators function.
	UserFunctionPrincipalInvestigator
	// UserFunctionTrainee for users that are trainees.
	UserFunctionTrainee
	// UserFunctionPhD for users that are PhD students.
	UserFunctionPhD
	// UserFunctionPostdoc for users that are Postdocs.
	UserFunctionPostdoc
	// UserFunctionResearchSupport for reseache support.
	UserFunctionResearchSupport
	// UserFunctionOtherSupport for other support staffs.
	UserFunctionOtherSupport
	// UserFunctionResearchStaff for research staffs.
	UserFunctionResearchStaff
	// UserFunctionOtherResearcher for general researchers.
	UserFunctionOtherResearcher
	// UserFunctionSeniorResearcher for senior researchers.
	UserFunctionSeniorResearcher
)

// UserStatus defines PDB user status.
type UserStatus int

const (
	// UserStatusUnknown refers to unexpected/unknown user status in PDB.
	UserStatusUnknown UserStatus = iota - 1
	// UserStatusCheckedIn refers to the status when the user is checked in.
	UserStatusCheckedIn
	// UserStatusCheckedOut refers to the status when the user has checked out.
	UserStatusCheckedOut
	// UserStatusCheckedOutExtended refers to the status when the user applied extended checkout.
	UserStatusCheckedOutExtended
	// UserStatusTentative refers to the status when the user is registered by not yet checked-in after following certain procedure.
	UserStatusTentative
)

// Lab defines an enumerator for the lab categories.
type Lab int

// Set implements the interface for flag.Var().
func (l *Lab) Set(v string) error {
	switch v {
	case "EEG":
		*l = EEG
	case "MEG":
		*l = MEG
	case "MRI":
		*l = MRI
	default:
		return fmt.Errorf("unknown modality: %s", v)
	}
	return nil
}

// String implements the interface for flag.Var().  It returns the
// name of the lab modality.
func (l *Lab) String() string {
	s := "unknown"
	switch *l {
	case EEG:
		s = "EEG"
	case MEG:
		s = "MEG"
	case MRI:
		s = "MRI"
	}
	return s
}

// GetDescriptionRegex returns a regular expression pattern for the description of
// a modality.
func (l *Lab) GetDescriptionRegex() (*regexp.Regexp, error) {
	switch *l {
	case EEG:
		return regexp.MustCompile(".*(EEG).*"), nil
	case MEG:
		return regexp.MustCompile(".*(MEG).*"), nil
	case MRI:
		return regexp.MustCompile(".*(SKYRA|PRISMA(FIT){0,1}).*"), nil
	default:
		return nil, fmt.Errorf("unknown modality: %s", l.String())
	}
}

const (
	// EEG is a lab modality of the EEG labs.
	EEG Lab = iota
	// MEG is a lab modality of the MEG labs.
	MEG
	// MRI is a lab modality of the MRI labs.
	MRI
)

// LabBooking defines the data structure of a booking event in the lab calendar.
type LabBooking struct {
	// Project is the id of the project to which the experiment belongs.
	Project string
	// Subject is the subject id of the participant.
	Subject string
	// Session is the session id of the participant.
	Session string
	// Modality is the experiment modality name.
	Modality string
	// Operator is the user operating the experiment.
	Operator User
	// ProjectTitle is the title of the project.
	ProjectTitle string
	// StartTime is the time the experiment starts.
	StartTime time.Time
}

// OpsIgnored is a specific error referring ignored operation.
type OpsIgnored struct {
	// Message is the detail information of the ignored operation.
	Message string
}

func (e *OpsIgnored) Error() string {
	return e.Message
}

// OoqLastAlert is the internal data structure.
type OoqLastAlert struct {
	// Timestamp is the moment the alert was sent.
	Timestamp time.Time
	// UsagePercent is the storage usage ratio in percent at the moment the alert was sent.
	UsagePercent int
	// UsagePercentLastCheck is the storage usage ratio at the last check.
	UsagePercentLastCheck int
}
