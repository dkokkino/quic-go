package congestion

import (
	"time"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
)

// Pacer is the public API for the token-bucket pacer.
// External congestion controllers compose a Pacer and delegate TimeUntilSend
// and HasPacingBudget to it.
//
// The getBandwidth function is called on each scheduling decision — return the
// current pacing rate in bytes/s. The pacer applies no multiplier of its own;
// callers are responsible for any desired gain (e.g. BBRv3 passes pacingRate
// directly, NewReno would pass bw * 5 / 4).
type Pacer struct {
	p *pacer
}

// NewPacer creates a Pacer with a caller-supplied bandwidth function.
// getBandwidth must return the current pacing rate in bytes per second.
func NewPacer(getBandwidth func() Bandwidth) *Pacer {
	p := &pacer{
		maxDatagramSize: initialMaxDatagramSize,
		adjustedBandwidth: func() uint64 {
			return uint64(getBandwidth() / BytesPerSecond)
		},
	}
	p.budgetAtLastSent = p.maxBurstSize()
	return &Pacer{p: p}
}

// TimeUntilSend returns when the next packet may be sent.
// Returns the zero time if a packet can be sent immediately.
func (p *Pacer) TimeUntilSend() time.Time {
	mt := p.p.TimeUntilSend()
	if mt == 0 {
		return time.Time{}
	}
	return mt.ToTime()
}

// Budget returns the number of bytes that may be sent at the given time.
func (p *Pacer) Budget(now time.Time) protocol.ByteCount {
	return p.p.Budget(monotime.FromTime(now))
}

// SentPacket must be called each time a packet is sent.
func (p *Pacer) SentPacket(sentTime time.Time, size protocol.ByteCount) {
	p.p.SentPacket(monotime.FromTime(sentTime), size)
}

// SetMaxDatagramSize updates the maximum datagram size used for burst calculations.
func (p *Pacer) SetMaxDatagramSize(s protocol.ByteCount) {
	p.p.SetMaxDatagramSize(s)
}
