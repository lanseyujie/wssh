package wssh

// options is an application options.
type options struct {
	user         string
	host         string
	port         int
	identityFile string
	password     string
	listenPort   int
}

// Option is an application option.
type Option func(o *options)

func User(user string) Option {
	return func(o *options) {
		o.user = user
	}
}

func Host(host string) Option {
	return func(o *options) {
		o.host = host
	}
}

func Port(port int) Option {
	return func(o *options) {
		o.port = port
	}
}

func IdentityFile(file string) Option {
	return func(o *options) {
		o.identityFile = file
	}
}

func Password(password string) Option {
	return func(o *options) {
		o.password = password
	}
}

func ListenPort(port int) Option {
	return func(o *options) {
		o.listenPort = port
	}
}
