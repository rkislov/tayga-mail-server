package smtp

import "testing"

func TestDomainOf(t *testing.T) {
	if domainOf("a@Ex.Com") != "ex.com" {
		t.Fatal(domainOf("a@Ex.Com"))
	}
	if domainOf("bad") != "" {
		t.Fatal("expected empty")
	}
}

func TestMXHostsFallback(t *testing.T) {
	// Invalid TLD should still return the domain as implicit MX fallback.
	hosts, err := mxHosts("this-domain-should-not-resolve-tayga-test.invalid")
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0] != "this-domain-should-not-resolve-tayga-test.invalid" {
		t.Fatalf("%v", hosts)
	}
}
