package pubsub

type SimpleQueueType int

const (
	SimpleQueueDurable SimpleQueueType = iota
	SimpleQueueTransient
)

var queueTypeEnum = map[SimpleQueueType]string{
	SimpleQueueDurable:   "durable",
	SimpleQueueTransient: "transient",
}
