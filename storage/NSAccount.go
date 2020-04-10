package storage

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// NSAccount Nintendo Switch account
type NSAccount struct {
	Name            string     `firestore:"name,omitempty"`
	NameInsensitive string     `firestore:"name_insensitive,omitempty"`
	FC              FriendCode `firestore:"friend_code,omitempty"`
}

// ParseAccountsFromString Parse FriendCode From String
func ParseAccountsFromString(msg, defaultname string) (accounts []NSAccount, err error) {
	var accountRegexp = regexp.MustCompile("^(?:(\\w+)\\s*:\\s*)?(?:[sS][wW]-?)?((?:\\d{12})|(?:\\d{4}-\\d{4}-\\d{4}))$")
	msg = strings.TrimSpace(msg) + ";"
	var substrs = strings.Split(msg, ";")
	for _, s := range substrs {
		submatchs := accountRegexp.FindAllStringSubmatch(strings.TrimSpace(s), 1)
		for _, m := range submatchs {
			code, err := strconv.ParseInt(strings.Replace(m[2], "-", "", -1), 10, 64)
			if err != nil {
				return accounts, fmt.Errorf("error: %v. wrong friend code format:%s", err, m[0])
			}
			var name string
			if len(m[1]) > 0 {
				name = m[1]
			} else {
				name = defaultname
			}
			accounts = append(accounts, NSAccount{name, strings.ToLower(name), FriendCode(code)})
		}
	}
	return accounts, nil
}

func (a NSAccount) String() string {
	if len(a.Name) == 0 {
		return a.FC.String()
	}
	return a.Name + ": " + a.FC.String()
}

// FriendCode is Nintendo Switch Friend Code
type FriendCode int64

func (fc FriendCode) String() string {
	c := int64(fc)
	return fmt.Sprintf("SW-%04d-%04d-%04d", c/100000000%10000, c/10000%10000, c%10000)
}
