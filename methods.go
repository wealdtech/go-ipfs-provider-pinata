package pinata

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"

	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	provider "github.com/wealdtech/go-ipfs-provider"
)

const (
	baseURL    = "https://api.pinata.cloud"
	gatewayURL = "https://gateway.pinata.cloud"
)

// Ping is an internal method to ensure the endpoint is accessible.
// This returns true if the endpoint is accessible, otherwise false.
// This returns an error for a network or authentication problem.
func (p *Provider) Ping() (bool, error) {
	res, err := p.get(fmt.Sprintf("%s/data/testAuthentication", baseURL), "")
	if err != nil {
		return false, err
	}

	if msg, exists := res["message"]; exists {
		if msg.(string) == "Congratulations! You are communicating with the Pinata API!" {
			return true, nil
		}
		return false, nil
	}

	return false, errors.New("unexpected failure")
}

// List lists all content pinned to this provider.
func (p *Provider) List() ([]*provider.ItemStatistics, error) {
	itemsPerPage := 100
	res, err := p.get(fmt.Sprintf("%s/data/pinList?status=pinned&pageLimit=%d&pageOffset=0", baseURL, itemsPerPage), "")
	if err != nil {
		return nil, err
	}

	count := 0
	if c, exists := res["count"]; exists {
		count = int(c.(float64))
	}
	content := make([]*provider.ItemStatistics, count)

	pages := count / itemsPerPage
	if count%itemsPerPage != 0 {
		pages++
	}
	for page := 0; page < pages; page++ {
		rows := res["rows"].([]interface{})
		for i := 0; i < len(rows) && page*itemsPerPage+i < len(content); i++ {
			row := rows[i].(map[string]interface{})
			size, err := strconv.ParseUint(row["size"].(string), 10, 64)
			if err != nil {
				return nil, err
			}
			item := &provider.ItemStatistics{
				Hash: row["ipfs_pin_hash"].(string),
				Size: size,
			}

			if name, exists := row["metadata"].(map[string]interface{})["name"]; exists {
				switch name.(type) {
				case string:
					item.Name = name.(string)
				}
			}

			content[page*itemsPerPage+i] = item
		}

		if page != pages-1 {
			res, err = p.get(fmt.Sprintf("%s/data/pinList?status=pinned&pageLimit=%d&pageOffset=%d", baseURL, itemsPerPage, page+1), "")
			if err != nil {
				return nil, err
			}
		}
	}

	return content, nil
}

// ItemStats returns information on an IPFS hash pinned to this provider.
func (p *Provider) ItemStats(hash string) (*provider.ItemStatistics, error) {
	res, err := p.get(fmt.Sprintf("%s/data/userPinList/hashContains/%s/pinStart/*/pinEnd/*/unpinStart/*/unpinEnd/*/pinSizeMin/*/pinSizeMax/*/pinFilter/pinned/pageLimit/1/pageOffset/0", baseURL, hash), "")
	if err != nil {
		return nil, err
	}

	count := 0
	if c, exists := res["count"]; exists {
		count = int(c.(float64))
	}
	if count == 0 {
		return nil, errors.New("unknown content")
	}
	if count > 1 {
		return nil, errors.New("multiple matching contents")
	}

	rows := res["rows"].([]interface{})
	row := rows[0].(map[string]interface{})
	size, err := strconv.ParseUint(row["size"].(string), 10, 64)
	if err != nil {
		return nil, err
	}
	item := &provider.ItemStatistics{
		Hash: row["ipfs_pin_hash"].(string),
		Size: size,
	}

	if name, exists := row["metadata"].(map[string]interface{})["name"]; exists {
		item.Name = name.(string)
	}

	return item, nil
}

// ServiceStats provides statistics for this provider.
func (p *Provider) ServiceStats() (*provider.SiteStatistics, error) {
	res, err := p.get(fmt.Sprintf("%s/data/userPinnedDataTotal", baseURL), "")
	if err != nil {
		return nil, err
	}

	items := uint64(0)
	if c, exists := res["pin_count"]; exists {
		items, err = strconv.ParseUint(c.(string), 10, 64)
		if err != nil {
			return nil, err
		}
	}

	size := uint64(0)
	if c, exists := res["pin_size_total"]; exists {
		size, err = strconv.ParseUint(c.(string), 10, 64)
		if err != nil {
			return nil, err
		}
	}

	return &provider.SiteStatistics{
		Items: items,
		Size:  size,
	}, nil
}

// PinContent pins content to this provider.
func (p *Provider) PinContent(name string, content io.Reader, opts *provider.ContentOpts) (string, error) {
	var b bytes.Buffer
	var contentType string

	if name != "" && content != nil {
		// Add content

		// Defer closing the content if it is closeable
		if x, ok := content.(io.Closer); ok {
			defer x.Close()
		}

		// Set up the form field
		w := multipart.NewWriter(&b)
		var fw io.Writer
		fw, err := w.CreateFormFile("file", name)
		if err != nil {
			return "", err
		}
		io.Copy(fw, content)

		if opts.StoreInDirectory {
			fw, err := w.CreateFormField("pinataOptions")
			if err != nil {
				return "", err
			}
			fw.Write([]byte(`{"wrapWithDirectory":true}`))
		}

		w.Close()
		contentType = w.FormDataContentType()
	}

	res, err := p.post(fmt.Sprintf("%s/pinning/pinFileToIPFS", baseURL), contentType, &b)
	if err != nil {
		return "", err
	}
	msg, exists := res["IpfsHash"]
	if exists {
		return msg.(string), nil
	}
	return "", errors.New("no hash returned")
}

// Pin pins existing IPFS content to this provider.
func (p *Provider) Pin(hash string) error {
	b := bytes.NewBufferString(fmt.Sprintf(`{"hashToPin":"%s"}`, hash))

	_, err := p.post(fmt.Sprintf("%s/pinning/pinHashToIPFS", baseURL), "application/json", b)
	return err
}

// Unpin removes content from this provider.
func (p *Provider) Unpin(hash string) error {
	b := bytes.NewBufferString(fmt.Sprintf(`{"ipfs_pin_hash":"%s"}`, hash))

	_, err := p.post(fmt.Sprintf("%s/pinning/removePinFromIPFS", baseURL), "application/json", b)
	return err
}

// GatewayURL provides a gateway URL for the given IPFS input
func (p *Provider) GatewayURL(input string) (string, error) {
	// Multiaddr
	_, err := ma.NewMultiaddr(input)
	if err == nil {
		return fmt.Sprintf("%s%s", gatewayURL, input), nil
	}

	// URI
	if strings.HasPrefix(input, "ipfs://") {
		return fmt.Sprintf("%s/ipfs/%s", gatewayURL, input[7:]), nil
	}
	if strings.HasPrefix(input, "ipns://") {
		return fmt.Sprintf("%s/ipns/%s", gatewayURL, input[7:]), nil
	}

	// Existing gateway URL
	index := strings.Index(input, "/ipfs/")
	if index == -1 {
		index = strings.Index(input, "/ipns/")
	}
	if index != -1 {
		return fmt.Sprintf("%s%s", gatewayURL, input[index:]), nil
	}

	// Plain hash
	_, err = mh.FromB58String(input)
	if err == nil {
		return fmt.Sprintf("%s/ipfs/%s", gatewayURL, input), nil
	}

	return "", errors.New("unrecognised format")
}
