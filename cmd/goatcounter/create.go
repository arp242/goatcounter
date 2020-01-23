// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

const usageCreate = `
Create a new site and user.

This is mostly useful for the "serve" command; if you're using "saas" you can
create a new site/user through the UI.

This will always create a site and user with the id of 1, and will overwrite any
existing site or user.

Required flags:

  -domain         Domain you'll be using, e.g. "example.com" or "stats.foo.com".

  -email          Your email address. This will be required to login.

  -name           The site's name; can be any string.
`

func create() error {
	return nil
}
