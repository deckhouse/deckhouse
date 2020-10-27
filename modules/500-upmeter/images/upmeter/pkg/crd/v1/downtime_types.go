package v1

import (
	"fmt"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"upmeter/pkg/probe/types"
)

type DowntimeSpec struct {
	StartDate   string   `json:"startDate"`
	EndDate     string   `json:"endDate"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Affected    []string `json:"affected"`
}

// DowntimeIncident is the Schema for the downtime incidents
type Downtime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec []DowntimeSpec `json:"spec,omitempty"`
}

// DowntimeList contains a list of DowntimeIncident
type DowntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Downtime `json:"items"`
}

// TODO use FilterFunc and store an array of DowntimeIncidents in a filterResult object.
func (d Downtime) GetDowntimeIncidents() []types.DowntimeIncident {
	res := make([]types.DowntimeIncident, 0)
	for _, obj := range d.Spec {
		start, err := DateToSeconds(obj.StartDate)
		if err != nil {
			log.Errorf("convert startDate '%s' in %s: %v", obj.StartDate, d.Name, err)
			continue
		}
		end, err := DateToSeconds(obj.EndDate)
		if err != nil {
			log.Errorf("convert endDate '%s' in %s: %v", obj.EndDate, d.Name, err)
			continue
		}
		inc := types.DowntimeIncident{
			Start:        start,
			End:          end,
			Duration:     0,
			Type:         obj.Type,
			Description:  obj.Description,
			Affected:     obj.Affected,
			DowntimeName: d.Name,
		}
		res = append(res, inc)
	}

	return res
}

func DateToSeconds(d string) (int64, error) {
	t, err := time.Parse(time.RFC3339, d)
	if err == nil {
		return t.Unix(), nil
	}
	seconds, errInt := strconv.ParseInt(d, 10, 32)
	if errInt == nil {
		return seconds, nil
	}

	return 0, fmt.Errorf("date is not a valid RFC3339 or Unix seconds")
}
