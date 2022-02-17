package spec

import (
	"encoding/json"
	"time"
)

// Override default JSON handling for TableMigration_NewColumns to handle AdditionalProperties
func (a *DateTime) UnmarshalJSON(b []byte) error {
	s := string(b)

	// Get rid of the quotes "" around the value.
	// A second option would be to include them
	// in the date format string instead, like so below:
	//   time.Parse(`"`+time.RFC3339Nano+`"`, s)
	s = s[1 : len(s)-1]

	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	*a = DateTime(t)
	return nil
}

// Override default JSON handling for TableMigration_NewColumns to handle AdditionalProperties
func (a DateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(a).Format(time.RFC3339))
}
