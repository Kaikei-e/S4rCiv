package kokkai

// JSON shapes of the kokkai (国会会議録検索API) responses we read. Only the
// fields M1 uses are typed; the canonical snapshot is hashed from the raw
// meetingRecord JSON, so adding fields here never shifts a content_hash.

import "encoding/json"

// listResponse is the shared envelope of meeting / meeting_list. The query-level
// fields (numberOfRecords, nextRecordPosition) are deliberately excluded from the
// snapshot: they describe the query, not the Resource.
type listResponse struct {
	NumberOfRecords    int               `json:"numberOfRecords"`
	NextRecordPosition int               `json:"nextRecordPosition"`
	MeetingRecord      []json.RawMessage `json:"meetingRecord"`
}

type meetingRecord struct {
	IssueID       string         `json:"issueID"`
	Session       int            `json:"session"`
	NameOfHouse   string         `json:"nameOfHouse"`
	NameOfMeeting string         `json:"nameOfMeeting"`
	Issue         string         `json:"issue"`
	Date          string         `json:"date"`
	MeetingURL    string         `json:"meetingURL"`
	SpeechRecord  []speechRecord `json:"speechRecord"`
}

type speechRecord struct {
	SpeechID        string `json:"speechID"`
	SpeechOrder     int    `json:"speechOrder"`
	Speaker         string `json:"speaker"`
	SpeakerYomi     string `json:"speakerYomi"`
	SpeakerGroup    string `json:"speakerGroup"`
	SpeakerPosition string `json:"speakerPosition"`
	Speech          string `json:"speech"`
	SpeechURL       string `json:"speechURL"`
}
