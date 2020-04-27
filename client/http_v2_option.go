package client

type HttpV2Option interface {
	apply(client *HttpV2Client)
}

func WithProtocol(s string) HttpV2Option {
	return &optionFunc{
		f: func(client *HttpV2Client) {
			client.Protocol = s
		},
	}
}

func WithPath(s string) HttpV2Option {
	return &optionFunc{
		f: func(client *HttpV2Client) {
			client.Path = s
		},
	}
}

func WithBoundary(s string) HttpV2Option {
	return &optionFunc{
		f: func(client *HttpV2Client) {
			client.boundary = s
		},
	}
}

func (fdo *optionFunc) apply(do *HttpV2Client) {
	fdo.f(do)
}
