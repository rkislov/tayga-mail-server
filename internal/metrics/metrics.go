package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/tayga/tms/internal/storage"
)

// Register installs Tayga collectors on the default Prometheus registry.
func Register(store storage.Driver) {
	c := &collector{store: store}
	if err := prometheus.Register(c); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
}

type collector struct {
	store storage.Driver

	mu    sync.Mutex
	cache *storage.ServerStats
	at    time.Time
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descTenants
	ch <- descDomains
	ch <- descUsers
	ch <- descUsersEnabled
	ch <- descMailboxes
	ch <- descMessages
	ch <- descBytes
	ch <- descCalendars
	ch <- descContacts
	ch <- descSieve
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	st := c.stats()
	if st == nil {
		return
	}
	ch <- prometheus.MustNewConstMetric(descTenants, prometheus.GaugeValue, float64(st.Tenants))
	ch <- prometheus.MustNewConstMetric(descDomains, prometheus.GaugeValue, float64(st.Domains))
	ch <- prometheus.MustNewConstMetric(descUsers, prometheus.GaugeValue, float64(st.Users))
	ch <- prometheus.MustNewConstMetric(descUsersEnabled, prometheus.GaugeValue, float64(st.UsersEnabled))
	ch <- prometheus.MustNewConstMetric(descMailboxes, prometheus.GaugeValue, float64(st.Mailboxes))
	ch <- prometheus.MustNewConstMetric(descMessages, prometheus.GaugeValue, float64(st.Messages))
	ch <- prometheus.MustNewConstMetric(descBytes, prometheus.GaugeValue, float64(st.BytesStored))
	ch <- prometheus.MustNewConstMetric(descCalendars, prometheus.GaugeValue, float64(st.Calendars))
	ch <- prometheus.MustNewConstMetric(descContacts, prometheus.GaugeValue, float64(st.Contacts))
	ch <- prometheus.MustNewConstMetric(descSieve, prometheus.GaugeValue, float64(st.SieveScripts))
}

func (c *collector) stats() *storage.ServerStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache != nil && time.Since(c.at) < 15*time.Second {
		return c.cache
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	st, err := c.store.ServerStats(ctx)
	if err != nil {
		return c.cache
	}
	c.cache = st
	c.at = time.Now()
	return st
}

var (
	descTenants      = prometheus.NewDesc("tayga_tenants", "Number of tenants", nil, nil)
	descDomains      = prometheus.NewDesc("tayga_domains", "Number of mail domains", nil, nil)
	descUsers        = prometheus.NewDesc("tayga_users", "Number of users", nil, nil)
	descUsersEnabled = prometheus.NewDesc("tayga_users_enabled", "Number of enabled users", nil, nil)
	descMailboxes    = prometheus.NewDesc("tayga_mailboxes", "Number of mailboxes", nil, nil)
	descMessages     = prometheus.NewDesc("tayga_messages", "Number of stored messages", nil, nil)
	descBytes        = prometheus.NewDesc("tayga_bytes_stored", "Total message bytes stored", nil, nil)
	descCalendars    = prometheus.NewDesc("tayga_calendars", "Number of calendars", nil, nil)
	descContacts     = prometheus.NewDesc("tayga_contacts", "Number of address book objects", nil, nil)
	descSieve        = prometheus.NewDesc("tayga_sieve_scripts", "Number of sieve scripts", nil, nil)
)
