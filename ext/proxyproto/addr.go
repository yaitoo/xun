package proxyproto

type Addr struct{ hp string }

func (a Addr) Network() string { return "tcp" }
func (a Addr) String() string  { return a.hp }
