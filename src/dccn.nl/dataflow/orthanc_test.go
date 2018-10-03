package orthanc

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dccn.nl/config"
	"github.com/spf13/viper"
)

var conf config.Configuration

func init() {
	// load configuration
	cfg := filepath.Join(os.Getenv("GOPATH"), "etc/config_test.yml")
	viper.SetConfigFile(cfg)
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Sprintf("Error reading config file, %s", err))
	}
	err := viper.Unmarshal(&conf)
	if err != nil {
		panic(fmt.Sprintf("unable to decode into struct, %s", err))
	}
}

func TestGetStudies(t *testing.T) {

	o := Orthanc{
		PrefixURL: conf.PACS.PrefixURL,
		Username:  conf.PACS.Username,
		Password:  conf.PACS.Password,
	}

	// get studies conducted in the last 24 hours
	studies, err := o.GetStudies(time.Now().Add(time.Hour*-24), time.Now())
	if err != nil {
		t.Errorf("Fail getting studies: %+v", err)
	}

	for _, s := range studies {
		d_s := s.MainDicomTags.StudyDate
		t_s := s.MainDicomTags.StudyTime
		dt_s := time.Date(d_s.Year(), d_s.Month(), d_s.Day(), t_s.Hour(), t_s.Minute(), t_s.Second(), 0, time.Now().Location())
		t.Logf("study %s, date: %s, nseries: %d", s.ID, dt_s, len(s.Series))

		// get first series
		if len(s.Series) > 0 {
			se, err := o.GetSeries(s.Series[0])
			if err != nil {
				t.Errorf("Fail getting series: %+v", err)
			}
			t.Logf("|- first series: %+v, last update: %s", se.ID, se.LastUpdate)
		}
	}
}

func TestListObjectIDs(t *testing.T) {

	o := Orthanc{
		PrefixURL: conf.PACS.PrefixURL,
		Username:  conf.PACS.Username,
		Password:  conf.PACS.Password,
	}

	// get objects acquired in the last 24 hours
	for _, l := range []DicomObject{DicomPatient, DicomStudy, DicomSeries} {
		objs, err := o.ListObjectIDs(l, time.Now().Add(time.Hour*-24), time.Now())
		if err != nil {
			t.Errorf("Fail getting %s: %+v", l, err)
		}

		for _, s := range objs {
			t.Logf("%s id: %s", l, s)
		}
	}

}
