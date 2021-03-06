package runner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/projectdiscovery/chaos-client/pkg/chaos"
	"github.com/projectdiscovery/gologger"
)

// RunEnumeration runs the enumeration for Chaos client
func RunEnumeration(opts *Options) {
	client := chaos.New(opts.APIKey)
	if opts.Update == "" && opts.hasStdin() {
		if _, err := client.PutSubdomains(&chaos.PutSubdomainsRequest{Contents: os.Stdin}); err != nil {
			gologger.Fatalf("Could not upload subdomains: %s\n", err)
		}
		gologger.Infof("Input processed successfully and subdomains with valid records will be updated to chaos dataset.")
		return
	}
	if opts.Update != "-" {
		file, err := os.Open(opts.Update)
		if err != nil {
			gologger.Fatalf("Could not open input file: %s\n", err)
		}
		defer file.Close()

		if _, err = client.PutSubdomains(&chaos.PutSubdomainsRequest{Contents: file}); err != nil {
			gologger.Fatalf("Could not upload subdomains: %s\n", err)
		}
		gologger.Infof("File processed successfully and subdomains with valid records will be updated to chaos dataset.")
		return
	}

	if opts.Count {
		resp, err := client.GetStatistics(&chaos.GetStatisticsRequest{
			Domain: opts.Domain,
		})
		if err != nil {
			gologger.Fatalf("Could not get statistics: %s\n", err)
		}
		gologger.Silentf("%d\n", resp.Subdomains)
		return
	}

	outputWriters := []io.Writer{os.Stdout}
	if opts.Output != "" {
		var err error
		opts.outputFile, err = os.Create(opts.Output)
		if err != nil {
			gologger.Fatalf("Could not create file %s for %s: %s\n", opts.Output, opts.Domain, err)
		}
		defer opts.outputFile.Close()
		outputWriters = append(outputWriters, opts.outputFile)
	}
	opts.outputWriter = io.MultiWriter(outputWriters...)

	if opts.Domain != "" {
		if opts.BBQ {
			processBBQDomain(client, opts)
		} else {
			processDomain(client, opts)
		}
	}

	if opts.hasStdin() || opts.DomainsFile != "" {
		processList(client, opts)
	}
}

func processDomain(client *chaos.Client, opts *Options) {
	req := &chaos.SubdomainsRequest{Domain: opts.Domain}
	if opts.JSONOutput {
		req.OutputFormat = "json"
	}
	for item := range client.GetSubdomains(req) {
		if item.Error != nil {
			gologger.Fatalf("Could not get subdomains: %s\n", item.Error)
		}
		if opts.JSONOutput {
			io.Copy(opts.outputWriter, *item.Reader)
		} else {
			if item.Subdomain != "" {
				gologger.Silentf("%s.%s\n", item.Subdomain, opts.Domain)
			}
			if opts.Output != "" {
				_, err := fmt.Fprintf(opts.outputWriter, "%s.%s\n", item.Subdomain, opts.Domain)
				if err != nil {
					gologger.Fatalf("Could not write results to file %s for %s: %s\n", opts.Output, opts.Domain, err)
				}
			}
		}
	}
}

func processBBQDomain(client *chaos.Client, opts *Options) {
	req := &chaos.SubdomainsRequest{Domain: opts.Domain}
	if opts.JSONOutput {
		req.OutputFormat = "json"
	}
	var outputLine string
	for item := range client.GetBBQSubdomains(req) {
		if item.Error != nil {
			gologger.Fatalf("Could not get subdomains: %s\n", item.Error)
		}
		var bbqdata chaos.BBQData
		if opts.JSONOutput {
			io.Copy(opts.outputWriter, *item.Reader)
		} else {
			if err := json.Unmarshal(item.Data, &bbqdata); err != nil {
				gologger.Fatalf("Could not unmarshal response: %s\n", err)
			}
			// filters
			if !applyFilter(&bbqdata, opts.filter) {
				continue
			}
			outputLine = fmt.Sprintf(extractOutput(&bbqdata, opts.filter))
			if outputLine != "" {
				_, err := fmt.Fprintf(opts.outputWriter, "%s\n", outputLine)
				if err != nil {
					gologger.Fatalf("Could not write results to file %s for %s: %s\n", opts.Output, opts.Domain, err)
				}
			}
		}
	}
}

func processList(client *chaos.Client, opts *Options) {
	var (
		file *os.File
		err  error
	)
	if opts.hasStdin() {
		file = os.Stdin
	} else if opts.Update != "" {
		file, err = os.Open(opts.DomainsFile)
		if err != nil {
			gologger.Fatalf("Could not open input file: %s\n", err)
		}
		defer file.Close()
	}

	in := bufio.NewScanner(file)
	for in.Scan() {
		opts.Domain = in.Text()
		if opts.BBQ {
			processBBQDomain(client, opts)
		} else {
			processDomain(client, opts)
		}
	}
}
