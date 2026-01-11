package soula

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"time"
)

// Timestamp 将time.Time转换为Unix时间戳的自定义类型
type Timestamp time.Time

// MarshalJSON 实现了自定义时间类型的 JSON 编组
func (t Timestamp) MarshalJSON() ([]byte, error) {
	timestamp := time.Time(t).UnixMilli()
	if timestamp <= 0 {
		timestamp = 0
	}
	return []byte(fmt.Sprintf(`%d`, timestamp)), nil
}

// UnmarshalJSON 方法来自定义解析时间字符串
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	intValue, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	timestamp := Timestamp(time.UnixMilli(intValue))
	*t = timestamp
	return nil
}

func (t Timestamp) Value() (driver.Value, error) {
	var zeroTime time.Time
	tlt := time.Time(t)
	if tlt.UnixNano() == zeroTime.UnixNano() {
		return nil, nil
	}
	return tlt, nil
}

// Scan 实现了 sql.Scanner 接口，用于从数据库读取数据到 Timestamp
func (t *Timestamp) Scan(v interface{}) error {
	if v == nil {
		return nil
	}
	switch vt := v.(type) {
	case time.Time:
		*t = Timestamp(vt)
	default:
		return fmt.Errorf("invalid type for timestamp: %T", v)
	}
	return nil
}

func (t Timestamp) IsZero() bool {
	return time.Time(t).IsZero()
}

func (t Timestamp) Time() time.Time {
	return time.Time(t)
}

func (t Timestamp) Duration() time.Duration {
	return time.Until(t.Time())
}
