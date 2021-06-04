package z18n

type Tagger interface {
	Open() string
	Close() string
}

type tag struct{ tag, content string }

func (t tag) Open() string {
	if t.content == "" {
		return "<" + t.tag + ">"
	}
	return "<" + t.tag + " " + t.content + ">"
}
func (t tag) Close() string { return "</" + t.tag + ">" }

type none struct{}

func (t none) Open() string  { return "" }
func (t none) Close() string { return "" }

func Tag(tagName, content string) tag { return tag{tag: tagName, content: content} }
func TagNone() Tagger                 { return none{} }
