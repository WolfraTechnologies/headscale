package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/juanfont/headscale/integration/hsic"
	"github.com/juanfont/headscale/integration/tsic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tailscale.com/tailcfg"
)

func TestResolveMagicDNS(t *testing.T) {
	IntegrationSkip(t)

	spec := ScenarioSpec{
		NodesPerUser: len(MustTestVersions) - 1,
		Users:        []string{"user1", "user2"},
		// The development HEAD client currently requires a newer Go toolchain
		// than its Dockerfile provides. Released versions cover this DNS change.
		Versions: MustTestVersions[1:],
	}

	scenario, err := NewScenario(spec)

	require.NoError(t, err)
	defer scenario.ShutdownAssertNoPanics(t)

	err = scenario.CreateHeadscaleEnv([]tsic.Option{}, hsic.WithTestName("magicdns"))
	requireNoErrHeadscaleEnv(t, err)

	allClients, err := scenario.ListTailscaleClients()
	requireNoErrListClients(t, err)

	err = scenario.WaitForTailscaleSync()
	requireNoErrSync(t, err)

	// assertClientsState(t, allClients)

	// Poor mans cache
	_, err = scenario.ListTailscaleClientsFQDNs()
	requireNoErrListFQDN(t, err)

	_, err = scenario.ListTailscaleClientsIPs()
	requireNoErrListClientIPs(t, err)

	expectedUsers := map[string]string{}

	for _, user := range spec.Users {
		clients, err := scenario.GetClients(user)
		require.NoError(t, err)

		for _, client := range clients {
			expectedUsers[client.Hostname()] = user
		}
	}

	for _, client := range allClients {
		for _, peer := range allClients {
			// It is safe to ignore this error as we handled it when caching it
			peerFQDN, _ := peer.FQDN()

			expectedFQDN := fmt.Sprintf("%s.%s.headscale.net.", peer.Hostname(), expectedUsers[peer.Hostname()])
			assert.Equal(t, expectedFQDN, peerFQDN)

			assert.EventuallyWithT(t, func(ct *assert.CollectT) {
				command := []string{
					"tailscale",
					"ip", peerFQDN,
				}
				result, _, err := client.Execute(command)
				assert.NoError(ct, err, "Failed to execute resolve/ip command %s from %s", peerFQDN, client.Hostname())

				ips, err := peer.IPs()
				assert.NoError(ct, err, "Failed to get IPs for %s", peer.Hostname())

				for _, ip := range ips {
					assert.Contains(ct, result, ip.String(), "IP %s should be found in DNS resolution result from %s to %s", ip.String(), client.Hostname(), peer.Hostname())
				}
			}, 30*time.Second, 2*time.Second)

			if client == allClients[0] {
				oldFQDN := peer.Hostname() + ".headscale.net."

				assert.EventuallyWithT(t, func(ct *assert.CollectT) {
					_, _, err := client.Execute([]string{"tailscale", "ip", oldFQDN})
					assert.Error(ct, err, "untagged base-domain alias %s should not resolve", oldFQDN)
				}, 30*time.Second, 2*time.Second)
			}
		}
	}
}

func TestResolveTaggedMagicDNS(t *testing.T) {
	IntegrationSkip(t)

	spec := ScenarioSpec{
		NodesPerUser: 1,
		Users:        []string{"user1"},
		Versions:     MustTestVersions[1:],
	}

	scenario, err := NewScenario(spec)

	require.NoError(t, err)
	defer scenario.ShutdownAssertNoPanics(t)

	err = scenario.createHeadscaleEnvWithTags(
		false,
		[]tsic.Option{},
		[]string{"tag:server"},
		"",
		hsic.WithTestName("taggedmagicdns"),
	)
	requireNoErrHeadscaleEnv(t, err)

	err = scenario.WaitForTailscaleSync()
	requireNoErrSync(t, err)

	allClients, err := scenario.ListTailscaleClients()
	requireNoErrListClients(t, err)
	require.Len(t, allClients, 1)
	client := allClients[0]

	var peerFQDN string

	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		peerFQDN, err = client.FQDN()
		assert.NoError(ct, err)
	}, 30*time.Second, 2*time.Second)

	expectedFQDN := client.Hostname() + ".headscale.net."
	assert.Equal(t, expectedFQDN, peerFQDN)

	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		result, _, err := client.Execute([]string{"tailscale", "ip", expectedFQDN})
		assert.NoError(ct, err)

		ips, err := client.IPs()
		assert.NoError(ct, err)

		for _, ip := range ips {
			assert.Contains(ct, result, ip.String())
		}
	}, 30*time.Second, 2*time.Second)

	userAlias := client.Hostname() + ".user1.headscale.net."

	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		_, _, err := client.Execute([]string{"tailscale", "ip", userAlias})
		assert.Error(ct, err, "tagged node must not receive a per-user alias %s", userAlias)
	}, 30*time.Second, 2*time.Second)
}

func TestResolveMagicDNSExtraRecordsPath(t *testing.T) {
	IntegrationSkip(t)

	spec := ScenarioSpec{
		NodesPerUser: 1,
		Users:        []string{"user1", "user2"},
	}

	scenario, err := NewScenario(spec)

	require.NoError(t, err)
	defer scenario.ShutdownAssertNoPanics(t)

	const erPath = "/tmp/extra_records.json"

	extraRecords := []tailcfg.DNSRecord{
		{
			Name:  "test.myvpn.example.com",
			Type:  "A",
			Value: "6.6.6.6",
		},
	}
	b, _ := json.Marshal(extraRecords)

	err = scenario.CreateHeadscaleEnv([]tsic.Option{
		tsic.WithPackages("python3", "curl", "bind-tools"),
	},
		hsic.WithTestName("extrarecords"),
		hsic.WithConfigEnv(map[string]string{
			// Disable global nameservers to make the test run offline.
			"HEADSCALE_DNS_NAMESERVERS_GLOBAL": "",
			"HEADSCALE_DNS_EXTRA_RECORDS_PATH": erPath,
		}),
		hsic.WithFileInContainer(erPath, b),
		hsic.WithEmbeddedDERPServerOnly(),
		hsic.WithTLS(),
	)
	requireNoErrHeadscaleEnv(t, err)

	allClients, err := scenario.ListTailscaleClients()
	requireNoErrListClients(t, err)

	err = scenario.WaitForTailscaleSync()
	requireNoErrSync(t, err)

	// assertClientsState(t, allClients)

	// Poor mans cache
	_, err = scenario.ListTailscaleClientsFQDNs()
	requireNoErrListFQDN(t, err)

	_, err = scenario.ListTailscaleClientsIPs()
	requireNoErrListClientIPs(t, err)

	for _, client := range allClients {
		assertCommandOutputContains(t, client, []string{"dig", "test.myvpn.example.com"}, "6.6.6.6")
	}

	hs, err := scenario.Headscale()
	require.NoError(t, err)

	// Write the file directly into place from the docker API.
	b0, _ := json.Marshal([]tailcfg.DNSRecord{
		{
			Name:  "docker.myvpn.example.com",
			Type:  "A",
			Value: "2.2.2.2",
		},
	})

	err = hs.WriteFile(erPath, b0)
	require.NoError(t, err)

	for _, client := range allClients {
		assertCommandOutputContains(t, client, []string{"dig", "docker.myvpn.example.com"}, "2.2.2.2")
	}

	// Write a new file and move it to the path to ensure the reload
	// works when a file is moved atomically into place.
	extraRecords = append(extraRecords, tailcfg.DNSRecord{
		Name:  "otherrecord.myvpn.example.com",
		Type:  "A",
		Value: "7.7.7.7",
	})
	b2, _ := json.Marshal(extraRecords)

	err = hs.WriteFile(erPath+"2", b2)
	require.NoError(t, err)
	_, err = hs.Execute([]string{"mv", erPath + "2", erPath})
	require.NoError(t, err)

	for _, client := range allClients {
		assertCommandOutputContains(t, client, []string{"dig", "test.myvpn.example.com"}, "6.6.6.6")
		assertCommandOutputContains(t, client, []string{"dig", "otherrecord.myvpn.example.com"}, "7.7.7.7")
	}

	// Write a new file and copy it to the path to ensure the reload
	// works when a file is copied into place.
	b3, _ := json.Marshal([]tailcfg.DNSRecord{
		{
			Name:  "copy.myvpn.example.com",
			Type:  "A",
			Value: "8.8.8.8",
		},
	})

	err = hs.WriteFile(erPath+"3", b3)
	require.NoError(t, err)
	_, err = hs.Execute([]string{"cp", erPath + "3", erPath})
	require.NoError(t, err)

	for _, client := range allClients {
		assertCommandOutputContains(t, client, []string{"dig", "copy.myvpn.example.com"}, "8.8.8.8")
	}

	// Write in place to ensure pipe like behaviour works
	b4, _ := json.Marshal([]tailcfg.DNSRecord{
		{
			Name:  "docker.myvpn.example.com",
			Type:  "A",
			Value: "9.9.9.9",
		},
	})
	command := []string{"echo", fmt.Sprintf("'%s'", string(b4)), ">", erPath}
	_, err = hs.Execute([]string{"bash", "-c", strings.Join(command, " ")})
	require.NoError(t, err)

	for _, client := range allClients {
		assertCommandOutputContains(t, client, []string{"dig", "docker.myvpn.example.com"}, "9.9.9.9")
	}

	// Delete the file and create a new one to ensure it is picked up again.
	_, err = hs.Execute([]string{"rm", erPath})
	require.NoError(t, err)

	// The same paths should still be available as it is not cleared on delete.
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		for _, client := range allClients {
			result, _, err := client.Execute([]string{"dig", "docker.myvpn.example.com"})
			assert.NoError(ct, err)
			assert.Contains(ct, result, "9.9.9.9")
		}
	}, 10*time.Second, 1*time.Second)

	// Write a new file, the backoff mechanism should make the filewatcher pick it up
	// again.
	err = hs.WriteFile(erPath, b3)
	require.NoError(t, err)

	for _, client := range allClients {
		assertCommandOutputContains(t, client, []string{"dig", "copy.myvpn.example.com"}, "8.8.8.8")
	}
}
