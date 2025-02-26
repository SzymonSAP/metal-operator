// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	metalv1alpha1 "github.com/ironcore-dev/metal-operator/api/v1alpha1"
	"github.com/ironcore-dev/metal-operator/bmc"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const DefaultKubeNamespace = "default"

func GetBMCClientForServer(ctx context.Context, c client.Client, server *metalv1alpha1.Server, insecure bool) (bmc.BMC, error) {
	if server.Spec.BMCRef != nil {
		b := &metalv1alpha1.BMC{}
		bmcName := server.Spec.BMCRef.Name
		if err := c.Get(ctx, client.ObjectKey{Name: bmcName}, b); err != nil {
			return nil, fmt.Errorf("failed to get BMC: %w", err)
		}

		return GetBMCClientFromBMC(ctx, c, b, insecure)
	}

	if server.Spec.BMC != nil {
		bmcSecret := &metalv1alpha1.BMCSecret{}
		if err := c.Get(ctx, client.ObjectKey{Name: server.Spec.BMC.BMCSecretRef.Name}, bmcSecret); err != nil {
			return nil, fmt.Errorf("failed to get BMC secret: %w", err)
		}

		return CreateBMCClient(
			ctx,
			c,
			insecure,
			server.Spec.BMC.Protocol.Name,
			server.Spec.BMC.Address,
			server.Spec.BMC.Protocol.Port,
			bmcSecret,
		)
	}

	return nil, fmt.Errorf("server %s has neither a BMCRef nor a BMC configured", server.Name)
}

func GetBMCClientFromBMC(ctx context.Context, c client.Client, bmcObj *metalv1alpha1.BMC, insecure bool) (bmc.BMC, error) {
	var address string

	if bmcObj.Spec.EndpointRef != nil {
		endpoint := &metalv1alpha1.Endpoint{}
		if err := c.Get(ctx, client.ObjectKey{Name: bmcObj.Spec.EndpointRef.Name}, endpoint); err != nil {
			return nil, fmt.Errorf("failed to get Endpoints for BMC: %w", err)
		}
		address = endpoint.Spec.IP.String()
	}

	if bmcObj.Spec.Endpoint != nil {
		address = bmcObj.Spec.Endpoint.IP.String()
	}

	bmcSecret := &metalv1alpha1.BMCSecret{}
	if err := c.Get(ctx, client.ObjectKey{Name: bmcObj.Spec.BMCSecretRef.Name}, bmcSecret); err != nil {
		return nil, fmt.Errorf("failed to get BMC secret: %w", err)
	}

	return CreateBMCClient(ctx, c, insecure, bmcObj.Spec.Protocol.Name, address, bmcObj.Spec.Protocol.Port, bmcSecret)
}

func CreateBMCClient(ctx context.Context, c client.Client, insecure bool, bmcProtocol metalv1alpha1.ProtocolName, address string, port int32, bmcSecret *metalv1alpha1.BMCSecret) (bmc.BMC, error) {
	protocol := "https"
	if insecure {
		protocol = "http"
	}

	var bmcClient bmc.BMC
	switch bmcProtocol {
	case metalv1alpha1.ProtocolRedfish:
		bmcAddress := fmt.Sprintf("%s://%s:%d", protocol, address, port)
		username, password, err := GetBMCCredentialsFromSecret(bmcSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to get credentials from BMC secret: %w", err)
		}
		bmcClient, err = bmc.NewRedfishBMCClient(ctx, bmcAddress, username, password, true)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redfish client: %w", err)
		}
	case metalv1alpha1.ProtocolRedfishLocal:
		bmcAddress := fmt.Sprintf("%s://%s:%d", protocol, address, port)
		username, password, err := GetBMCCredentialsFromSecret(bmcSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to get credentials from BMC secret: %w", err)
		}
		bmcClient, err = bmc.NewRedfishLocalBMCClient(ctx, bmcAddress, username, password, true)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redfish client: %w", err)
		}
	case metalv1alpha1.ProtocolRedfishKube:
		bmcAddress := fmt.Sprintf("%s://%s:%d", protocol, address, port)
		username, password, err := GetBMCCredentialsFromSecret(bmcSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to get credentials from BMC secret: %w", err)
		}
		bmcClient, err = bmc.NewRedfishKubeBMCClient(ctx, bmcAddress, username, password, true, c, DefaultKubeNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redfish client: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported BMC protocol %s", bmcProtocol)
	}
	return bmcClient, nil
}

func GetBMCCredentialsFromSecret(secret *metalv1alpha1.BMCSecret) (string, string, error) {
	// TODO: use constants for secret keys
	username, ok := secret.Data["username"]
	if !ok {
		return "", "", fmt.Errorf("no username found in the BMC secret")
	}
	password, ok := secret.Data["password"]
	if !ok {
		return "", "", fmt.Errorf("no password found in the BMC secret")
	}
	return string(username), string(password), nil
}

func GetServerNameFromBMCandIndex(index int, bmc *metalv1alpha1.BMC) string {
	return fmt.Sprintf("%s-%s-%d", bmc.Name, "system", index)
}
