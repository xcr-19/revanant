package main

import (
	"github.com/MythicMeta/MythicContainer"
	revanantfunctions "github.com/xcr-19/revanant/Payload_Type/revanant/container"
)

func main() {
	revanantfunctions.Initialize()

	MythicContainer.StartAndRunForever([]MythicContainer.MythicServices{
		MythicContainer.MythicServicePayload,
		MythicContainer.MythicServiceC2,
	})

}
