package ackhandler

import (
	"testing"
	"time"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
	"github.com/stretchr/testify/require"
)

type hookTrackingCongestion struct {
	maxDatagramSize protocol.ByteCount
	rttStats        *utils.RTTStats

	ackStartTimes         []time.Time
	ackStartBytesInFlight []protocol.ByteCount
	ackEndTimes           []time.Time
	lossDetectionStarts   int
	// lossDetectionStarts value at the time each OnCongestionEvent fired — used to
	// assert that OnLossDetectionStart always precedes any congestion signal.
	congestionEventAfterLossStart []int
	ecnAckedBytes         []protocol.ByteCount
	ecnECT0               []int64
	ecnECT1               []int64
	ecnCE                 []int64
	ecnPriorInFlight      []protocol.ByteCount
	ptoBytesInFlight      []protocol.ByteCount
	spuriousPackets       []protocol.PacketNumber
	spuriousReordering    []protocol.PacketNumber
	migrationSizes        []protocol.ByteCount
	appLimitedBytes []protocol.ByteCount
}

func (*hookTrackingCongestion) TimeUntilSend(protocol.ByteCount) time.Time { return time.Time{} }
func (*hookTrackingCongestion) HasPacingBudget(time.Time) bool             { return true }
func (*hookTrackingCongestion) OnPacketSent(time.Time, protocol.ByteCount, protocol.PacketNumber, protocol.ByteCount, bool) {
}
func (*hookTrackingCongestion) CanSend(protocol.ByteCount) bool { return true }
func (*hookTrackingCongestion) MaybeExitSlowStart()             {}
func (*hookTrackingCongestion) OnPacketAcked(protocol.PacketNumber, protocol.ByteCount, protocol.ByteCount, time.Time) {
}
func (h *hookTrackingCongestion) OnCongestionEvent(_ protocol.PacketNumber, _ protocol.ByteCount, _ protocol.ByteCount) {
	h.congestionEventAfterLossStart = append(h.congestionEventAfterLossStart, h.lossDetectionStarts)
}
func (h *hookTrackingCongestion) SetMaxDatagramSize(s protocol.ByteCount) {
	h.maxDatagramSize = s
}
func (*hookTrackingCongestion) InSlowStart() bool { return false }
func (*hookTrackingCongestion) InRecovery() bool  { return false }
func (h *hookTrackingCongestion) GetCongestionWindow() protocol.ByteCount {
	return h.maxDatagramSize * 10
}
func (h *hookTrackingCongestion) SetRTTStats(rttStats *utils.RTTStats) { h.rttStats = rttStats }
func (h *hookTrackingCongestion) OnAckEventStart(eventTime time.Time, bytesInFlight protocol.ByteCount) {
	h.ackStartTimes = append(h.ackStartTimes, eventTime)
	h.ackStartBytesInFlight = append(h.ackStartBytesInFlight, bytesInFlight)
}
func (h *hookTrackingCongestion) OnAckEventEnd(eventTime time.Time) {
	h.ackEndTimes = append(h.ackEndTimes, eventTime)
}
func (h *hookTrackingCongestion) OnLossDetectionStart() { h.lossDetectionStarts++ }
func (h *hookTrackingCongestion) OnECNCongestion(
	ackedBytes protocol.ByteCount,
	ect0Total, ect1Total, ceTotal int64,
	priorInFlight protocol.ByteCount,
	_ time.Time,
) {
	h.ecnAckedBytes = append(h.ecnAckedBytes, ackedBytes)
	h.ecnECT0 = append(h.ecnECT0, ect0Total)
	h.ecnECT1 = append(h.ecnECT1, ect1Total)
	h.ecnCE = append(h.ecnCE, ceTotal)
	h.ecnPriorInFlight = append(h.ecnPriorInFlight, priorInFlight)
}
func (h *hookTrackingCongestion) MarkAppLimited(bytesInFlight protocol.ByteCount) {
	h.appLimitedBytes = append(h.appLimitedBytes, bytesInFlight)
}
func (h *hookTrackingCongestion) OnSpuriousLossDetected(packetNumber, packetReordering protocol.PacketNumber) {
	h.spuriousPackets = append(h.spuriousPackets, packetNumber)
	h.spuriousReordering = append(h.spuriousReordering, packetReordering)
}
func (h *hookTrackingCongestion) OnPTO(bytesInFlight protocol.ByteCount) {
	h.ptoBytesInFlight = append(h.ptoBytesInFlight, bytesInFlight)
}
func (h *hookTrackingCongestion) OnConnectionMigration(initialMaxDatagramSize protocol.ByteCount) {
	h.migrationSizes = append(h.migrationSizes, initialMaxDatagramSize)
	h.maxDatagramSize = initialMaxDatagramSize
}

type fallbackOnlyCongestion struct {
	maxDatagramSize protocol.ByteCount
}

func (*fallbackOnlyCongestion) TimeUntilSend(protocol.ByteCount) time.Time { return time.Time{} }
func (*fallbackOnlyCongestion) HasPacingBudget(time.Time) bool             { return true }
func (*fallbackOnlyCongestion) OnPacketSent(time.Time, protocol.ByteCount, protocol.PacketNumber, protocol.ByteCount, bool) {
}
func (*fallbackOnlyCongestion) CanSend(protocol.ByteCount) bool { return true }
func (*fallbackOnlyCongestion) MaybeExitSlowStart()             {}
func (*fallbackOnlyCongestion) OnPacketAcked(protocol.PacketNumber, protocol.ByteCount, protocol.ByteCount, time.Time) {
}
func (*fallbackOnlyCongestion) OnCongestionEvent(protocol.PacketNumber, protocol.ByteCount, protocol.ByteCount) {
}
func (f *fallbackOnlyCongestion) SetMaxDatagramSize(s protocol.ByteCount) {
	f.maxDatagramSize = s
}
func (*fallbackOnlyCongestion) InSlowStart() bool { return false }
func (*fallbackOnlyCongestion) InRecovery() bool  { return false }
func (f *fallbackOnlyCongestion) GetCongestionWindow() protocol.ByteCount {
	return f.maxDatagramSize * 10
}

func TestSentPacketHandlerBindsCustomControllerState(t *testing.T) {
	rttStats := utils.NewRTTStats()
	cong := &hookTrackingCongestion{}

	sph := NewSentPacketHandler(
		0,
		1234,
		rttStats,
		&utils.ConnectionStats{},
		false,
		false,
		nil,
		protocol.PerspectiveClient,
		nil,
		cong,
		utils.DefaultLogger,
	)

	handler := sph.(*sentPacketHandler)
	require.False(t, handler.usesDefaultCongestion)
	require.Same(t, cong, handler.congestion)
	require.Same(t, rttStats, cong.rttStats)
	require.Equal(t, protocol.ByteCount(1234), cong.maxDatagramSize)
}

func TestSentPacketHandlerMigrationHookForCustomController(t *testing.T) {
	cong := &hookTrackingCongestion{}
	sph := NewSentPacketHandler(
		0,
		1200,
		utils.NewRTTStats(),
		&utils.ConnectionStats{},
		false,
		false,
		nil,
		protocol.PerspectiveClient,
		nil,
		cong,
		utils.DefaultLogger,
	)

	handler := sph.(*sentPacketHandler)
	handler.MigratedPath(monotime.Now(), 1400)

	require.Same(t, cong, handler.congestion)
	require.Equal(t, []protocol.ByteCount{1400}, cong.migrationSizes)
}

func TestSentPacketHandlerMigrationFallsBackWithoutHook(t *testing.T) {
	cong := &fallbackOnlyCongestion{}
	sph := NewSentPacketHandler(
		0,
		1200,
		utils.NewRTTStats(),
		&utils.ConnectionStats{},
		false,
		false,
		nil,
		protocol.PerspectiveClient,
		nil,
		cong,
		utils.DefaultLogger,
	)

	handler := sph.(*sentPacketHandler)
	handler.MigratedPath(monotime.Now(), 1400)

	require.NotSame(t, cong, handler.congestion)
}

func TestSentPacketHandlerNotifiesAckLossAndECNHooks(t *testing.T) {
	now := monotime.Now()
	cong := &hookTrackingCongestion{}
	sph := NewSentPacketHandler(
		0,
		1200,
		utils.NewRTTStats(),
		&utils.ConnectionStats{},
		false,
		true, // enableECN
		nil,
		protocol.PerspectiveClient,
		nil,
		cong,
		utils.DefaultLogger,
	)

	// Send and ACK numECNTestingPackets (10) to advance ecnTracker to ecnStateCapable.
	// ECNMode must be called before each send to trigger the ecnTracker's testing phase,
	// matching what connection.go does in production.
	// OnECNCongestion only fires once the path is validated.
	handler := sph.(*sentPacketHandler)
	for i := range numECNTestingPackets {
		handler.ECNMode(true) // triggers ecnStateTesting and marks packets as testing
		pn := sph.PopPacketNumber(protocol.Encryption1RTT)
		sph.SentPacket(now.Add(time.Duration(i)*time.Millisecond), pn, protocol.InvalidPacketNumber, nil, []Frame{{Frame: &wire.PingFrame{}}}, protocol.Encryption1RTT, protocol.ECT0, 1200, false, false)
		_, err := sph.ReceivedAck(&wire.AckFrame{
			AckRanges: []wire.AckRange{{Smallest: pn, Largest: pn}},
			ECT0:      uint64(i + 1),
		}, protocol.Encryption1RTT, now.Add(time.Duration(i)*time.Millisecond+20*time.Millisecond))
		require.NoError(t, err)
	}
	cong.ecnAckedBytes = nil
	cong.ecnECT0 = nil
	cong.ecnECT1 = nil
	cong.ecnCE = nil
	cong.ecnPriorInFlight = nil
	cong.lossDetectionStarts = 0
	cong.ackStartTimes = nil
	cong.ackEndTimes = nil
	cong.ackStartBytesInFlight = nil

	// Now send one more packet and ACK it — ecnTracker is capable, OnECNCongestion fires.
	pn := sph.PopPacketNumber(protocol.Encryption1RTT)
	sph.SentPacket(now.Add(100*time.Millisecond), pn, protocol.InvalidPacketNumber, nil, []Frame{{Frame: &wire.PingFrame{}}}, protocol.Encryption1RTT, protocol.ECT0, 1200, false, false)

	acked, err := sph.ReceivedAck(&wire.AckFrame{
		AckRanges: []wire.AckRange{{Smallest: pn, Largest: pn}},
		ECT0:      uint64(numECNTestingPackets + 1),
	}, protocol.Encryption1RTT, now.Add(120*time.Millisecond))
	require.NoError(t, err)
	require.True(t, acked)

	require.Equal(t, 1, cong.lossDetectionStarts)
	require.Len(t, cong.ackStartTimes, 1)
	require.Len(t, cong.ackEndTimes, 1)
	require.Equal(t, []protocol.ByteCount{1200}, cong.ackStartBytesInFlight)
	require.Equal(t, []protocol.ByteCount{1200}, cong.ecnAckedBytes)
	require.Equal(t, []int64{numECNTestingPackets + 1}, cong.ecnECT0)
	require.Equal(t, []int64{0}, cong.ecnECT1)
	require.Equal(t, []int64{0}, cong.ecnCE)
	require.Equal(t, []protocol.ByteCount{1200}, cong.ecnPriorInFlight)
}

func TestSentPacketHandlerNotifiesPTOHook(t *testing.T) {
	now := monotime.Now()
	cong := &hookTrackingCongestion{}
	sph := NewSentPacketHandler(
		0,
		1200,
		utils.NewRTTStats(),
		&utils.ConnectionStats{},
		false,
		false,
		nil,
		protocol.PerspectiveClient,
		nil,
		cong,
		utils.DefaultLogger,
	)

	pn := sph.PopPacketNumber(protocol.EncryptionInitial)
	sph.SentPacket(now, pn, protocol.InvalidPacketNumber, nil, []Frame{{Frame: &wire.PingFrame{}}}, protocol.EncryptionInitial, protocol.ECNNon, 1200, false, false)

	timeout := sph.GetLossDetectionTimeout()
	require.NotZero(t, timeout)
	require.NoError(t, sph.OnLossDetectionTimeout(timeout))
	require.Equal(t, []protocol.ByteCount{1200}, cong.ptoBytesInFlight)
}

// TestSentPacketHandlerAckEventStartReceivesPostLossBytesInFlight verifies that
// OnAckEventStart receives bytesInFlight *after* lost packets have been removed.
// BBRv3 relies on this for accurate phase-transition decisions (e.g. entering
// ProbeRTT only when inflight has genuinely drained). If OnAckEventStart fired
// before loss detection, it would observe inflated bytesInFlight and make incorrect
// state transitions.
func TestSentPacketHandlerAckEventStartReceivesPostLossBytesInFlight(t *testing.T) {
	now := monotime.Now()
	cong := &hookTrackingCongestion{}
	sph := NewSentPacketHandler(
		0,
		1200,
		utils.NewRTTStats(),
		&utils.ConnectionStats{},
		false,
		false,
		nil,
		protocol.PerspectiveClient,
		nil,
		cong,
		utils.DefaultLogger,
	)

	// Send 5 packets (5 * 1200 = 6000 bytes in flight).
	for i := range 5 {
		pn := sph.PopPacketNumber(protocol.Encryption1RTT)
		sph.SentPacket(now.Add(time.Duration(i)*time.Millisecond), pn, protocol.InvalidPacketNumber, nil,
			[]Frame{{Frame: &wire.PingFrame{}}}, protocol.Encryption1RTT, protocol.ECNNon, 1200, false, false)
	}

	// ACK only packet 4. With the default reorder threshold of 3, packet 0 is
	// more than 3 packet numbers behind the largest acked and is declared lost.
	// Expected bytesInFlight at OnAckEventStart:
	//   6000 (total sent) - 1200 (packet 4 acked) - 1200 (packet 0 lost) = 3600
	_, err := sph.ReceivedAck(&wire.AckFrame{
		AckRanges: []wire.AckRange{{Smallest: 4, Largest: 4}},
	}, protocol.Encryption1RTT, now.Add(50*time.Millisecond))
	require.NoError(t, err)

	require.Len(t, cong.ackStartBytesInFlight, 1)
	require.Equal(t, protocol.ByteCount(3600), cong.ackStartBytesInFlight[0],
		"OnAckEventStart must receive post-loss bytesInFlight (lost packet already removed)")
}

// TestSentPacketHandlerLossDetectionStartPrecedesAllCongestionEvents verifies
// that OnLossDetectionStart fires before any OnCongestionEvent call within a
// single ACK — for both packet-loss-driven and ECN-driven congestion signals.
// Controllers use OnLossDetectionStart to reset per-pass counters (e.g. RFC §5.3.1.3
// "count at most one loss event per detection pass"). Receiving a congestion signal
// before that reset would corrupt the counter.
func TestSentPacketHandlerLossDetectionStartPrecedesAllCongestionEvents(t *testing.T) {
	now := monotime.Now()
	cong := &hookTrackingCongestion{}
	sph := NewSentPacketHandler(
		0,
		1200,
		utils.NewRTTStats(),
		&utils.ConnectionStats{},
		false,
		false,
		nil,
		protocol.PerspectiveClient,
		nil,
		cong,
		utils.DefaultLogger,
	)

	// Send 5 packets; ACK only the last one so packets 0-3 are declared lost,
	// and also set ECN CE to trigger an ECN-driven OnCongestionEvent.
	for i := range 5 {
		pn := sph.PopPacketNumber(protocol.Encryption1RTT)
		sph.SentPacket(now.Add(time.Duration(i)*time.Millisecond), pn, protocol.InvalidPacketNumber, nil,
			[]Frame{{Frame: &wire.PingFrame{}}}, protocol.Encryption1RTT, protocol.ECT0, 1200, false, false)
	}

	// Ack only packet 4; packets 0-3 exceed the reorder threshold (default 3) and are lost.
	// ECT0=4, CE=1 triggers the ecnTracker to report congestion.
	_, err := sph.ReceivedAck(&wire.AckFrame{
		AckRanges: []wire.AckRange{{Smallest: 4, Largest: 4}},
		ECT0:      4,
		ECNCE:     1,
	}, protocol.Encryption1RTT, now.Add(50*time.Millisecond))
	require.NoError(t, err)

	require.NotEmpty(t, cong.congestionEventAfterLossStart,
		"expected at least one OnCongestionEvent (loss or ECN)")
	for i, lossStartCount := range cong.congestionEventAfterLossStart {
		require.Greater(t, lossStartCount, 0,
			"OnCongestionEvent[%d] fired before OnLossDetectionStart", i)
	}
}

func TestSentPacketHandlerNotifiesSpuriousLossHook(t *testing.T) {
	now := monotime.Now()
	cong := &hookTrackingCongestion{}
	sph := NewSentPacketHandler(
		0,
		1200,
		utils.NewRTTStats(),
		&utils.ConnectionStats{},
		false,
		false,
		nil,
		protocol.PerspectiveClient,
		nil,
		cong,
		utils.DefaultLogger,
	)

	var firstSendTime monotime.Time
	for i := range 4 {
		pn := sph.PopPacketNumber(protocol.Encryption1RTT)
		sendTime := now.Add(time.Duration(i) * time.Millisecond)
		if i == 0 {
			firstSendTime = sendTime
		}
		sph.SentPacket(sendTime, pn, protocol.InvalidPacketNumber, nil, []Frame{{Frame: &wire.PingFrame{}}}, protocol.Encryption1RTT, protocol.ECNNon, 1200, false, false)
	}

	handler := sph.(*sentPacketHandler)
	handler.lostPackets.Add(0, firstSendTime)
	handler.detectSpuriousLosses(
		&wire.AckFrame{
			AckRanges: []wire.AckRange{
				{Smallest: 4, Largest: 4},
				{Smallest: 0, Largest: 0},
			},
		},
		now.Add(10*time.Millisecond),
	)

	require.Equal(t, []protocol.PacketNumber{0}, cong.spuriousPackets)
	require.Equal(t, []protocol.PacketNumber{4}, cong.spuriousReordering)
}
