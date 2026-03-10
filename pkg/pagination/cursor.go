package pagination

import (
	"simple-securities/pkg/conversion"
	"time"
)

type Cursor struct {
	Id        uint64
	CreatedAt time.Time
}

func NewCursor(id uint64, createdAt time.Time) *Cursor {
	return &Cursor{
		Id:        id,
		CreatedAt: createdAt,
	}
}

func (c *Cursor) Encode() string {
	val, _ := conversion.ToJSON(c)
	return conversion.Base64StrEncode(val)
}

func DecodeCursor(encoded string) (*Cursor, error) {
	decoded, err := conversion.Base64StrDecode(encoded)
	if err != nil {
		return nil, err
	}

	var c Cursor
	if err = conversion.FromJSON(decoded, &c); err != nil {
		return nil, err
	}

	return &c, nil
}
