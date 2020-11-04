package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http/httputil"
	"net/url"

	"github.com/crowdsecurity/crowdsec/pkg/apiclient"
	"github.com/crowdsecurity/crowdsec/pkg/csconfig"
	"github.com/crowdsecurity/crowdsec/pkg/cwhub"
	"github.com/crowdsecurity/crowdsec/pkg/cwversion"
	"github.com/crowdsecurity/crowdsec/pkg/models"
	"github.com/go-openapi/strfmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var LAPIURLPrefix string = "v1"

func NewLapiCmd() *cobra.Command {
	var cmdLapi = &cobra.Command{
		Use:   "lapi [action]",
		Short: "Manage interraction with Local API (LAPI)",
		Args:  cobra.MinimumNArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if csConfig.API.Client == nil {
				log.Fatalln("There is no API->client configuration")
			}
			if csConfig.API.Client.Credentials == nil {
				log.Fatalf("no configuration for crowdsec API in '%s'", *csConfig.Self)
			}
			return nil
		},
	}

	var cmdLapiRegister = &cobra.Command{
		Use:   "register",
		Short: "Register a machine to Local API (LAPI)",
		Long: `Register you machine to the Local API (LAPI).
Keep in mind the machine needs to be validated by an administrator on LAPI side to be effective.`,
		Args: cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			id, err := generateID()
			if err != nil {
				log.Fatalf("unable to generate machine id: %s", err)
			}
			password := strfmt.Password(generatePassword(passwordLength))
			if apiURL == "" {
				if csConfig.API.Client != nil && csConfig.API.Client.Credentials != nil && csConfig.API.Client.Credentials.URL != "" {
					apiURL = csConfig.API.Client.Credentials.URL
				} else {
					log.Fatalf("No Local API URL. Please provide it in your configuration or with the -u parameter")
				}
			}
			apiurl, err := url.Parse(apiURL)
			if err != nil {
				log.Fatalf("parsing api url: %s", err)
			}
			_, err = apiclient.RegisterClient(&apiclient.Config{
				MachineID:     id,
				Password:      password,
				UserAgent:     fmt.Sprintf("crowdsec/%s", cwversion.VersionStr()),
				URL:           apiurl,
				VersionPrefix: LAPIURLPrefix,
			}, nil)

			if err != nil {
				log.Fatalf("api client register: %s", err)
			}

			var dumpFile string
			if outputFile != "" {
				dumpFile = outputFile
			} else if csConfig.API.Client.CredentialsFilePath != "" {
				dumpFile = csConfig.API.Client.CredentialsFilePath
			} else {
				dumpFile = ""
			}
			apiCfg := csconfig.ApiCredentialsCfg{
				Login:    id,
				Password: password.String(),
				URL:      apiURL,
			}
			apiConfigDump, err := yaml.Marshal(apiCfg)
			if err != nil {
				log.Fatalf("unable to marshal api credentials: %s", err)
			}
			if dumpFile != "" {
				err = ioutil.WriteFile(dumpFile, apiConfigDump, 0644)
				if err != nil {
					log.Fatalf("write api credentials in '%s' failed: %s", dumpFile, err)
				}
				log.Printf("API credentials dumped to '%s'", dumpFile)
			} else {
				fmt.Printf("%s\n", string(apiConfigDump))
			}
			log.Warningf("Run 'systemctl reload crowdsec' for the new configuration to be effective")
		},
	}
	cmdLapiRegister.Flags().StringVarP(&apiURL, "url", "u", "", "URL of the API")
	cmdLapiRegister.Flags().StringVarP(&outputFile, "file", "f", "", "output file destination")
	cmdLapi.AddCommand(cmdLapiRegister)

	var cmdLapiStatus = &cobra.Command{
		Use:   "status",
		Short: "Check authentication to Local API (LAPI)",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			password := strfmt.Password(csConfig.API.Client.Credentials.Password)
			apiurl, err := url.Parse(csConfig.API.Client.Credentials.URL)
			login := csConfig.API.Client.Credentials.Login
			if err != nil {
				log.Fatalf("parsing api url ('%s'): %s", apiurl, err)
			}
			if err := cwhub.GetHubIdx(csConfig.Cscli); err != nil {
				log.Fatalf("Failed to load hub index : %s", err)
			}
			scenarios, err := cwhub.GetUpstreamInstalledScenariosAsString()
			if err != nil {
				log.Fatalf("failed to get scenarios : %s", err.Error())
			}

			Client, err = apiclient.NewDefaultClient(apiurl,
				LAPIURLPrefix,
				fmt.Sprintf("crowdsec/%s", cwversion.VersionStr()),
				nil)
			if err != nil {
				log.Fatalf("init default client: %s", err)
			}
			t := models.WatcherAuthRequest{
				MachineID: &login,
				Password:  &password,
				Scenarios: scenarios,
			}
			log.Infof("Loaded credentials from %s", csConfig.API.Client.CredentialsFilePath)
			resp, err := Client.Auth.AuthenticateWatcher(context.Background(), t)
			if err != nil {
				log.Fatalf("Failed to authenticate to Local API (LAPI) : %s", err)
			} else {
				log.Infof("You can successfully interact with Local API (LAPI)")
			}
			for k, v := range resp.Response.Header {
				log.Debugf("[headers] %s : %s", k, v)
			}
			dump, _ := httputil.DumpResponse(resp.Response, true)
			log.Debugf("Response: %s", string(dump))
		},
	}
	cmdLapi.AddCommand(cmdLapiStatus)
	return cmdLapi
}
