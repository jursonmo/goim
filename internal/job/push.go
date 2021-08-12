package job

import (
	"context"
	"fmt"

	"github.com/Terry-Mao/goim/api/comet"
	pb "github.com/Terry-Mao/goim/api/logic"
	"github.com/Terry-Mao/goim/api/protocol"
	"github.com/Terry-Mao/goim/pkg/bytes"
	log "github.com/golang/glog"
)

func (j *Job) push(ctx context.Context, pushMsg *pb.PushMsg) (err error) {
	switch pushMsg.Type {
	case pb.PushMsg_PUSH:
		err = j.pushKeys(pushMsg.Operation, pushMsg.Server, pushMsg.Keys, pushMsg.Msg)
	case pb.PushMsg_ROOM:
		err = j.getRoom(pushMsg.Room).Push(pushMsg.Operation, pushMsg.Msg)
	case pb.PushMsg_BROADCAST:
		err = j.broadcast(pushMsg.Operation, pushMsg.Msg, pushMsg.Speed)
	default:
		err = fmt.Errorf("no match push type: %s", pushMsg.Type)
	}
	return
}

// pushKeys push a message to a batch of subkeys.
func (j *Job) pushKeys(operation int32, serverID string, subKeys []string, body []byte) (err error) {
	buf := bytes.NewWriterSize(len(body) + 64)
	p := &protocol.Proto{
		Ver:  1,
		Op:   operation,
		Body: body,
	}
	p.WriteTo(buf)        //把 p 按 pktLen:4 headLen:2 Ver:2 op:4 seq:4 格式组装进buf
	p.Body = buf.Buffer() //然后把这个组装后的buf 赋值给p.Body, 也就是p.Body 就是完整的协议报文了, 这样comet 拿到protocol.OpRaw的数据后，可以直接转发p.Body的数据给client,不需要再次组装。
	p.Op = protocol.OpRaw
	var args = comet.PushMsgReq{
		Keys:    subKeys,
		ProtoOp: operation,
		Proto:   p,
	}
	if c, ok := j.cometServers[serverID]; ok {
		if err = c.Push(&args); err != nil {
			log.Errorf("c.Push(%v) serverID:%s error(%v)", args, serverID, err)
		}
		log.Infof("pushKey:%s comets:%d", serverID, len(j.cometServers))
	}
	return
}

// broadcast broadcast a message to all.
func (j *Job) broadcast(operation int32, body []byte, speed int32) (err error) {
	buf := bytes.NewWriterSize(len(body) + 64)
	p := &protocol.Proto{
		Ver:  1,
		Op:   operation,
		Body: body,
	}
	p.WriteTo(buf)
	p.Body = buf.Buffer()
	p.Op = protocol.OpRaw
	comets := j.cometServers
	speed /= int32(len(comets))
	var args = comet.BroadcastReq{
		ProtoOp: operation,
		Proto:   p,
		Speed:   speed,
	}
	for serverID, c := range comets {
		if err = c.Broadcast(&args); err != nil {
			log.Errorf("c.Broadcast(%v) serverID:%s error(%v)", args, serverID, err)
		}
	}
	log.Infof("broadcast comets:%d", len(comets))
	return
}

// broadcastRoomRawBytes broadcast aggregation messages to room.
func (j *Job) broadcastRoomRawBytes(roomID string, body []byte) (err error) {
	args := comet.BroadcastRoomReq{
		RoomID: roomID,
		Proto: &protocol.Proto{
			Ver:  1,
			Op:   protocol.OpRaw,
			Body: body,
		},
	}
	comets := j.cometServers
	for serverID, c := range comets {
		if err = c.BroadcastRoom(&args); err != nil {
			log.Errorf("c.BroadcastRoom(%v) roomID:%s serverID:%s error(%v)", args, roomID, serverID, err)
		}
	}
	log.Infof("broadcastRoom comets:%d", len(comets))
	return
}
