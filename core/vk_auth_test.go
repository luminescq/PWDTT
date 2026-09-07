package core

import (
	"errors"
	"fmt"
	"testing"
)

func TestShouldRetryVKCallsOutdatedToken(t *testing.T) {
	outdated := fmt.Errorf("error.webrtc.auth.anonym_token.outdated")
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "step five outdated token",
			err:  newVKCallsFailure("step5 vchat.joinConversationByLink", vkCallsFailureOKCDN, outdated),
			want: true,
		},
		{
			name: "outdated token before step five",
			err:  newVKCallsFailure("step4 auth.anonymLogin", vkCallsFailureOKCDN, outdated),
			want: false,
		},
		{
			name: "other step five error",
			err:  newVKCallsFailure("step5 vchat.joinConversationByLink", vkCallsFailureOKCDN, errors.New("temporary failure")),
			want: false,
		},
		{
			name: "unwrapped error",
			err:  outdated,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRetryVKCallsOutdatedToken(tc.err); got != tc.want {
				t.Errorf("shouldRetryVKCallsOutdatedToken() = %v, want %v", got, tc.want)
			}
		})
	}
}
