package haproxy

import "fmt"

func (c *Client) Watcher() {
	fmt.Println(c.client.GetStats().Stats)
}
