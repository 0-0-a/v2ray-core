// Package kcp - A Fast and Reliable ARQ Protocol
//
// Acknowledgement:
//    skywind3000@github for inventing the KCP protocol
//    xtaci@github for translating to Golang
package kcp

import (
	"github.com/v2ray/v2ray-core/common/alloc"
	"github.com/v2ray/v2ray-core/common/log"
)

const (
	IKCP_RTO_NDL  = 30  // no delay min rto
	IKCP_RTO_MIN  = 100 // normal min rto
	IKCP_RTO_DEF  = 200
	IKCP_RTO_MAX  = 60000
	IKCP_WND_SND  = 32
	IKCP_WND_RCV  = 32
	IKCP_INTERVAL = 100
)

func _itimediff(later, earlier uint32) int32 {
	return (int32)(later - earlier)
}

type State int

const (
	StateActive       State = 0
	StateReadyToClose State = 1
	StatePeerClosed   State = 2
	StateTerminating  State = 3
	StateTerminated   State = 4
)

// KCP defines a single KCP connection
type KCP struct {
	conv             uint16
	state            State
	stateBeginTime   uint32
	lastIncomingTime uint32
	lastPayloadTime  uint32
	sendingUpdated   bool
	receivingUpdated bool
	lastPingTime     uint32

	mss                             uint32
	snd_una, snd_nxt, rcv_nxt       uint32
	rx_rttvar, rx_srtt, rx_rto      uint32
	snd_wnd, rcv_wnd, rmt_wnd, cwnd uint32
	current, interval               uint32

	snd_queue *SendingQueue
	rcv_queue *ReceivingQueue
	snd_buf   *SendingWindow
	rcv_buf   *ReceivingWindow

	acklist *ACKList

	fastresend        int32
	congestionControl bool
	output            *SegmentWriter
}

// NewKCP create a new kcp control object, 'conv' must equal in two endpoint
// from the same connection.
func NewKCP(conv uint16, output *AuthenticationWriter) *KCP {
	log.Debug("KCP|Core: creating KCP ", conv)
	kcp := new(KCP)
	kcp.conv = conv
	kcp.snd_wnd = effectiveConfig.GetSendingWindowSize()
	kcp.rcv_wnd = effectiveConfig.GetReceivingWindowSize()
	kcp.rmt_wnd = IKCP_WND_RCV
	kcp.mss = output.Mtu() - DataSegmentOverhead
	kcp.rx_rto = IKCP_RTO_DEF
	kcp.interval = IKCP_INTERVAL
	kcp.output = NewSegmentWriter(output)
	kcp.rcv_buf = NewReceivingWindow(effectiveConfig.GetReceivingWindowSize())
	kcp.snd_queue = NewSendingQueue(effectiveConfig.GetSendingQueueSize())
	kcp.rcv_queue = NewReceivingQueue()
	kcp.acklist = NewACKList(kcp)
	kcp.snd_buf = NewSendingWindow(kcp, effectiveConfig.GetSendingWindowSize())
	kcp.cwnd = kcp.snd_wnd
	return kcp
}

func (kcp *KCP) SetState(state State) {
	kcp.state = state
	kcp.stateBeginTime = kcp.current

	switch state {
	case StateReadyToClose:
		kcp.rcv_queue.Close()
	case StatePeerClosed:
		kcp.ClearSendQueue()
	case StateTerminating:
		kcp.rcv_queue.Close()
	case StateTerminated:
		kcp.rcv_queue.Close()
	}
}

func (kcp *KCP) HandleOption(opt SegmentOption) {
	if (opt & SegmentOptionClose) == SegmentOptionClose {
		kcp.OnPeerClosed()
	}
}

func (kcp *KCP) OnPeerClosed() {
	if kcp.state == StateReadyToClose {
		kcp.SetState(StateTerminating)
	}
	if kcp.state == StateActive {
		kcp.SetState(StatePeerClosed)
	}
}

func (kcp *KCP) OnClose() {
	if kcp.state == StateActive {
		kcp.SetState(StateReadyToClose)
	}
	if kcp.state == StatePeerClosed {
		kcp.SetState(StateTerminating)
	}
}

// DumpReceivingBuf moves available data from rcv_buf -> rcv_queue
// @Private
func (kcp *KCP) DumpReceivingBuf() {
	for {
		seg := kcp.rcv_buf.RemoveFirst()
		if seg == nil {
			break
		}
		kcp.rcv_queue.Put(seg.Data)
		seg.Data = nil

		kcp.rcv_buf.Advance()
		kcp.rcv_nxt++
		kcp.receivingUpdated = true
	}
}

// Send is user/upper level send, returns below zero for error
func (kcp *KCP) Send(buffer []byte) int {
	nBytes := 0
	for len(buffer) > 0 && !kcp.snd_queue.IsFull() {
		var size int
		if len(buffer) > int(kcp.mss) {
			size = int(kcp.mss)
		} else {
			size = len(buffer)
		}
		seg := &DataSegment{
			Data: alloc.NewSmallBuffer().Clear().Append(buffer[:size]),
		}
		kcp.snd_queue.Push(seg)
		buffer = buffer[size:]
		nBytes += size
	}
	return nBytes
}

// https://tools.ietf.org/html/rfc6298
func (kcp *KCP) update_ack(rtt int32) {
	if kcp.rx_srtt == 0 {
		kcp.rx_srtt = uint32(rtt)
		kcp.rx_rttvar = uint32(rtt) / 2
	} else {
		delta := rtt - int32(kcp.rx_srtt)
		if delta < 0 {
			delta = -delta
		}
		kcp.rx_rttvar = (3*kcp.rx_rttvar + uint32(delta)) / 4
		kcp.rx_srtt = (7*kcp.rx_srtt + uint32(rtt)) / 8
		if kcp.rx_srtt < kcp.interval {
			kcp.rx_srtt = kcp.interval
		}
	}
	var rto uint32
	if kcp.interval < 4*kcp.rx_rttvar {
		rto = kcp.rx_srtt + 4*kcp.rx_rttvar
	} else {
		rto = kcp.rx_srtt + kcp.interval
	}

	if rto > IKCP_RTO_MAX {
		rto = IKCP_RTO_MAX
	}
	kcp.rx_rto = rto * 3 / 2
}

func (kcp *KCP) shrink_buf() {
	prevUna := kcp.snd_una
	if kcp.snd_buf.Len() > 0 {
		seg := kcp.snd_buf.First()
		kcp.snd_una = seg.Number
	} else {
		kcp.snd_una = kcp.snd_nxt
	}
	if kcp.snd_una != prevUna {
		kcp.sendingUpdated = true
	}
}

func (kcp *KCP) parse_ack(sn uint32) {
	if _itimediff(sn, kcp.snd_una) < 0 || _itimediff(sn, kcp.snd_nxt) >= 0 {
		return
	}

	kcp.snd_buf.Remove(sn - kcp.snd_una)
}

func (kcp *KCP) parse_fastack(sn uint32) {
	if _itimediff(sn, kcp.snd_una) < 0 || _itimediff(sn, kcp.snd_nxt) >= 0 {
		return
	}

	kcp.snd_buf.HandleFastAck(sn)
}

func (kcp *KCP) HandleReceivingNext(receivingNext uint32) {
	kcp.snd_buf.Clear(receivingNext)
}

func (kcp *KCP) HandleSendingNext(sendingNext uint32) {
	kcp.acklist.Clear(sendingNext)
}

func (kcp *KCP) parse_data(newseg *DataSegment) {
	sn := newseg.Number
	if _itimediff(sn, kcp.rcv_nxt+kcp.rcv_wnd) >= 0 ||
		_itimediff(sn, kcp.rcv_nxt) < 0 {
		return
	}

	idx := sn - kcp.rcv_nxt
	if !kcp.rcv_buf.Set(idx, newseg) {
		newseg.Release()
	}

	kcp.DumpReceivingBuf()
}

// Input when you received a low level packet (eg. UDP packet), call it
func (kcp *KCP) Input(data []byte) int {
	kcp.lastIncomingTime = kcp.current

	var seg ISegment
	var maxack uint32
	var flag int
	for {
		seg, data = ReadSegment(data)
		if seg == nil {
			break
		}

		switch seg := seg.(type) {
		case *DataSegment:
			kcp.HandleOption(seg.Opt)
			kcp.HandleSendingNext(seg.SendingNext)
			kcp.acklist.Add(seg.Number, seg.Timestamp)
			kcp.receivingUpdated = true
			kcp.parse_data(seg)
			kcp.lastPayloadTime = kcp.current
		case *ACKSegment:
			kcp.HandleOption(seg.Opt)
			if kcp.rmt_wnd < seg.ReceivingWindow {
				kcp.rmt_wnd = seg.ReceivingWindow
			}
			kcp.HandleReceivingNext(seg.ReceivingNext)
			for i := 0; i < int(seg.Count); i++ {
				ts := seg.TimestampList[i]
				sn := seg.NumberList[i]
				if _itimediff(kcp.current, ts) >= 0 {
					kcp.update_ack(_itimediff(kcp.current, ts))
				}
				kcp.parse_ack(sn)
				kcp.shrink_buf()
				if flag == 0 {
					flag = 1
					maxack = sn
				} else if _itimediff(sn, maxack) > 0 {
					maxack = sn
				}
			}
			kcp.lastPayloadTime = kcp.current
		case *CmdOnlySegment:
			kcp.HandleOption(seg.Opt)
			if seg.Cmd == SegmentCommandTerminated {
				if kcp.state == StateActive ||
					kcp.state == StateReadyToClose ||
					kcp.state == StatePeerClosed {
					kcp.SetState(StateTerminating)
				} else if kcp.state == StateTerminating {
					kcp.SetState(StateTerminated)
				}
			}
			kcp.HandleReceivingNext(seg.ReceivinNext)
			kcp.HandleSendingNext(seg.SendingNext)
			kcp.shrink_buf()
		default:
		}
	}

	if flag != 0 {
		kcp.parse_fastack(maxack)
	}

	return 0
}

// flush pending data
func (kcp *KCP) flush() {
	if kcp.state == StateTerminated {
		return
	}
	if kcp.state == StateActive && _itimediff(kcp.current, kcp.lastPayloadTime) >= 30000 {
		kcp.OnClose()
	}

	if kcp.state == StateTerminating {
		kcp.output.Write(&CmdOnlySegment{
			Conv: kcp.conv,
			Cmd:  SegmentCommandTerminated,
		})
		kcp.output.Flush()

		if _itimediff(kcp.current, kcp.stateBeginTime) > 8000 {
			kcp.SetState(StateTerminated)
		}
		return
	}

	if kcp.state == StateReadyToClose && _itimediff(kcp.current, kcp.stateBeginTime) > 15000 {
		kcp.SetState(StateTerminating)
	}

	current := kcp.current

	// flush acknowledges
	if kcp.acklist.Flush() {
		kcp.receivingUpdated = false
	}

	// calculate window size
	cwnd := kcp.snd_una + kcp.snd_wnd
	if cwnd < kcp.rmt_wnd {
		cwnd = kcp.rmt_wnd
	}
	if kcp.congestionControl && cwnd < kcp.snd_una+kcp.cwnd {
		cwnd = kcp.snd_una + kcp.cwnd
	}

	for !kcp.snd_queue.IsEmpty() && _itimediff(kcp.snd_nxt, cwnd) < 0 {
		seg := kcp.snd_queue.Pop()
		seg.Conv = kcp.conv
		seg.Number = kcp.snd_nxt
		seg.timeout = current
		seg.ackSkipped = 0
		seg.transmit = 0
		kcp.snd_buf.Push(seg)
		kcp.snd_nxt++
	}

	// flush data segments
	if kcp.snd_buf.Flush() {
		kcp.sendingUpdated = false
	}

	if kcp.sendingUpdated || kcp.receivingUpdated || _itimediff(kcp.current, kcp.lastPingTime) >= 5000 {
		seg := &CmdOnlySegment{
			Conv:         kcp.conv,
			Cmd:          SegmentCommandPing,
			ReceivinNext: kcp.rcv_nxt,
			SendingNext:  kcp.snd_una,
		}
		if kcp.state == StateReadyToClose {
			seg.Opt = SegmentOptionClose
		}
		kcp.output.Write(seg)
		kcp.lastPingTime = kcp.current
		kcp.sendingUpdated = false
		kcp.receivingUpdated = false
	}

	// flash remain segments
	kcp.output.Flush()

}

func (kcp *KCP) HandleLost(lost bool) {
	if !kcp.congestionControl {
		return
	}
	if lost {
		kcp.cwnd = 3 * kcp.cwnd / 4
	} else {
		kcp.cwnd += kcp.cwnd / 4
	}
	if kcp.cwnd < 4 {
		kcp.cwnd = 4
	}
	if kcp.cwnd > kcp.snd_wnd {
		kcp.cwnd = kcp.snd_wnd
	}
}

// Update updates state (call it repeatedly, every 10ms-100ms), or you can ask
// ikcp_check when to call it again (without ikcp_input/_send calling).
// 'current' - current timestamp in millisec.
func (kcp *KCP) Update(current uint32) {
	kcp.current = current
	kcp.flush()
}

// NoDelay options
// fastest: ikcp_nodelay(kcp, 1, 20, 2, 1)
// nodelay: 0:disable(default), 1:enable
// interval: internal update timer interval in millisec, default is 100ms
// resend: 0:disable fast resend(default), 1:enable fast resend
// nc: 0:normal congestion control(default), 1:disable congestion control
func (kcp *KCP) NoDelay(interval uint32, resend int, congestionControl bool) int {
	kcp.interval = interval

	if resend >= 0 {
		kcp.fastresend = int32(resend)
	}
	kcp.congestionControl = congestionControl
	return 0
}

// WaitSnd gets how many packet is waiting to be sent
func (kcp *KCP) WaitSnd() uint32 {
	return uint32(kcp.snd_buf.Len()) + kcp.snd_queue.Len()
}

func (this *KCP) ClearSendQueue() {
	this.snd_queue.Clear()
	this.snd_buf.Clear(0xFFFFFFFF)
}
