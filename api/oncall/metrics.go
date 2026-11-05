package oncall

import "github.com/prometheus/client_golang/prometheus"

var (
    incidentsFired = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "observatory_oncall_incidents_fired_total",
        Help: "Total number of on‑call incidents created.",
    })
    incidentsAcknowledged = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "observatory_oncall_incidents_acknowledged_total",
        Help: "Total number of incidents acknowledged.",
    })
    incidentsResolved = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "observatory_oncall_incidents_resolved_total",
        Help: "Total number of incidents resolved.",
    })
)

func Register() {
    prometheus.MustRegister(incidentsFired, incidentsAcknowledged, incidentsResolved)
}
