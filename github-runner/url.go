// Based on github.com/cli/go-gh/v2, which has the following MIT license.
//
// Copyright (c) 2021 GitHub Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

func parseAddress(address string) (*url.URL, error) {
	u, err := url.Parse(normalizeAddress(address))
	if err != nil {
		return nil, err
	}

	if u.Hostname() == "" {
		return nil, fmt.Errorf("missing hostname: %s", address)
	}

	if (u.Scheme == "ssh" || u.Scheme == "git+ssh") && strings.HasPrefix(u.Path, "//") {
		u.Path = strings.TrimPrefix(u.Path, "/")
	}

	return u, nil
}

var schemes = []string{"file", "ftp", "ftps", "git", "git+https", "git+ssh", "http", "https", "ssh"}

func normalizeAddress(address string) string {
	hasScheme := slices.ContainsFunc(schemes, func(s string) bool {
		return strings.HasPrefix(address, s+":")
	})
	if hasScheme {
		return address
	}

	// Looks like an SSH address and not a Windows path.
	if strings.ContainsRune(address, ':') && !strings.ContainsRune(address, '\\') {
		return "ssh://" + strings.Replace(address, ":", "/", 1)
	}

	return address
}

func parseRepository(u *url.URL) (string, string, string, error) {
	hostname := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) < 2 {
		return "", "", "", fmt.Errorf("too few segments: %s", u.Path)
	}
	if len(segments) > 2 {
		return "", "", "", fmt.Errorf("too many segments: %s", u.Path)
	}

	owner := segments[0]
	repo := strings.TrimSuffix(segments[1], ".git")

	return hostname, owner, repo, nil
}
