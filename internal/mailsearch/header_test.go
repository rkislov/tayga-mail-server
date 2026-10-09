package mailsearch

import "testing"

func TestDecodeHeader(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"=?UTF-8?B?0J/RgNC40LLQtdGC?=", "Привет"},
		{"=?windows-1251?B?z/Do4uXy?=", "Привет"},
		{"=?utf-8?Q?Hello?=\r\n =?utf-8?Q?_world?=", "Hello world"},
		{"=?invalid?B?broken?=", "=?invalid?B?broken?="},
		{"Plain subject", "Plain subject"},
		{"=?utf-8?B?RmFsY29udHJhZGVzLm5ldA===?= <info@falcontrades.net>", "Falcontrades.net <info@falcontrades.net>"},
	} {
		if got := DecodeHeader(tc.raw); got != tc.want {
			t.Errorf("DecodeHeader(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
