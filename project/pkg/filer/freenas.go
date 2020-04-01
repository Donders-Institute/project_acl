package filer

// FreeNasConfig implements the `Config` interface and extends it with configurations
// that are specific to the FreeNas filer.
type FreeNasConfig struct {
	// ApiURL is the server URL of the OnTAP APIs.
	ApiURL string
	// ApiUser is the username for the basic authentication of the OnTAP API.
	ApiUser string
	// ApiPass is the password for the basic authentication of the OnTAP API.
	ApiPass string
	// ProjectRoot specifies the top-level NAS path in which projects are located.
	ProjectRoot string

	// ProjectUser specifies the system username for the owner of the project directory.
	ProjectUser string
	// ProjectGID specifies the system groupname for the owner of the project directory.
	ProjectGroup string
}

// GetApiURL returns the server URL of the OnTAP API.
func (c FreeNasConfig) GetApiURL() string { return c.ApiURL }

// GetApiUser returns the username for the API basic authentication.
func (c FreeNasConfig) GetApiUser() string { return c.ApiUser }

// GetApiPass returns the password for the API basic authentication.
func (c FreeNasConfig) GetApiPass() string { return c.ApiPass }

// GetProjectRoot returns the filesystem root path in which directories of projects are located.
func (c FreeNasConfig) GetProjectRoot() string { return c.ProjectRoot }

type FreeNas struct {
	config FreeNasConfig
}

func (filer FreeNas) CreateProject(projectID string, quotaGiB int) error {
	return nil
}

func (filer FreeNas) CreateHome(username, groupname string, quotaGiB int) error {
	return nil
}

func (filer FreeNas) SetProjectQuota(projectID string, quotaGiB int) error {
	return nil
}

func (filer FreeNas) SetHomeQuota(username, groupname string, quotaGiB int) error {
	return nil
}

func (filer FreeNas) GetProjectQuotaInBytes(projectID string) (int64, error) {
	return 0, nil
}

func (filer FreeNas) GetHomeQuotaInBytes(username, groupname string) (int64, error) {
	return 0, nil
}
