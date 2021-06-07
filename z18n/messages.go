package z18n

import (
	"github.com/BurntSushi/toml"
	"zgo.at/errors"
	"zgo.at/goatcounter/z18n/finder"
	"zgo.at/zstd/zfilepath"
)

func ReadMessages(file string) (map[string]finder.Entry, error) {
	_, ext := zfilepath.SplitExt(file)

	var (
		m   map[string]finder.Entry
		err error
	)
	switch ext {
	default:
		return nil, errors.Errorf("unknown file type: %q", ext)
	case "json":
		// TODO
	case "go":
		// TODO
	case "gettext":
		// TODO
	case "toml":
		_, err = toml.DecodeFile(file, &m)
	}
	// TODO: is this really needed?
	if m != nil {
		for k, v := range m {
			v.ID = k
			m[k] = v
		}
	}
	return m, err
}
