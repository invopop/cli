package sequence

type Series struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Prefix      string `json:"prefix,omitempty"`
	Padding     int32  `json:"padding,omitempty"`
	Suffix      string `json:"suffix,omitempty"`
	LastIndex   int64  `json:"last_index,omitempty"`
	LastEntryID string `json:"last_entry_id,omitempty"`
}

type SeriesCollection struct {
	List []*Series `json:"list"`
}

type Entry struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}
