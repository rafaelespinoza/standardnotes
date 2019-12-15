package encryption

import (
	"crypto/sha1"
	"fmt"
	"os"
	"strings"
)

// SigningKey - export to routing
var SigningKey = []byte{}

func init() {
	key := os.Getenv("SECRET_KEY_BASE")
	if key == "" {
		key = "qA6irmDikU6RkCM4V0cJiUJEROuCsqTa1esexI4aWedSv405v8lw4g1KB1nQVsSdCrcyRlKFdws4XPlsArWwv9y5Xr5Jtkb11w1NxKZabOUa7mxjeENuCs31Y1Ce49XH9kGMPe0ms7iV7e9F6WgnsPFGOlIA3CwfGyr12okas2EsDd71SbSnA0zJYjyxeCVCZJWISmLB"
	}
	SigningKey = []byte(key)
}

func Salt(email, nonce string) string {
	return strings.Replace(
		fmt.Sprintf("% x", sha1.Sum([]byte(email+"SN"+nonce))),
		" ",
		"",
		-1,
	)
}
