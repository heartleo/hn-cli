package algolia

// item mirrors the JSON shape returned by https://hn.algolia.com/api/v1/items/{id}.
// Fields that may be null on dead/deleted comments are pointers so we can
// distinguish "missing" from "empty string". Only the subset used by the
// comment tree is mapped — scores, titles, urls etc. are ignored here
// because story metadata still comes from Firebase.
type item struct {
	ID         int     `json:"id"`
	CreatedAtI int64   `json:"created_at_i"`
	Type       string  `json:"type"`
	Author     *string `json:"author"`
	Text       *string `json:"text"`
	ParentID   *int    `json:"parent_id"`
	StoryID    *int    `json:"story_id"`
	Children   []item  `json:"children"`
}
