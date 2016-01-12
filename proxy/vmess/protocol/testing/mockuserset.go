package mocks

import (
	"github.com/v2ray/v2ray-core/proxy/vmess"
	"github.com/v2ray/v2ray-core/proxy/vmess/protocol"
)

type MockUserSet struct {
	Users      []vmess.User
	UserHashes map[string]int
	Timestamps map[string]protocol.Timestamp
}

func (us *MockUserSet) AddUser(user vmess.User) error {
	us.Users = append(us.Users, user)
	return nil
}

func (us *MockUserSet) GetUser(userhash []byte) (vmess.User, protocol.Timestamp, bool) {
	idx, found := us.UserHashes[string(userhash)]
	if found {
		return us.Users[idx], us.Timestamps[string(userhash)], true
	}
	return nil, 0, false
}
