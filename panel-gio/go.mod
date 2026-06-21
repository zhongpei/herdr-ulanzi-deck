module github.com/herdr-deck/herdrdeck/panel-gio

go 1.26.2

require (
	gioui.org v0.10.0
	github.com/herdr-deck/herdrdeck/displaymodel v0.0.0-00010101000000-000000000000
	github.com/herdr-deck/herdrdeck/protocol v0.0.0-00010101000000-000000000000
	github.com/nats-io/nats.go v1.52.0
	github.com/rs/zerolog v1.35.1
	github.com/spf13/cobra v1.10.2
)

require (
	gioui.org/shader v1.0.8 // indirect
	github.com/go-text/typesetting v0.3.4 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/nats-io/nkeys v0.4.16 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/exp v0.0.0-20251017212417-90e834f514db // indirect
	golang.org/x/exp/shiny v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/image v0.39.0 // indirect
	golang.org/x/net v0.54.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace (
	github.com/herdr-deck/herdrdeck/displaymodel => ../displaymodel
	github.com/herdr-deck/herdrdeck/protocol => ../protocol
)
