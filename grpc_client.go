package grpcoveriap

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"crypto/tls"
	"crypto/x509"

	"golang.stackrox.io/grpc-http1/client"
	"google.golang.org/api/iam/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const tlsPort = 443

// TODO: This JWT should probably be cached!
func signJWTWithGcloudDefaultCredentials(serviceAccount, targetUrl string) (string, error) {
	ctx := context.Background()

	// 1. Generate the JWT payload
	iat := time.Now().Unix()
	exp := iat + 3600 // 1 hour expiration

	payload := map[string]any{
		"iss": serviceAccount,
		"sub": serviceAccount,
		"aud": targetUrl,
		"iat": iat,
		"exp": exp,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWT payload: %w", err)
	}

	iamService, err := iam.NewService(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create IAM service client: %w", err)
	}

	serviceAccountResourceName := "projects/-/serviceAccounts/" + serviceAccount

	signJwtReq := &iam.SignJwtRequest{
		Payload: string(payloadJSON), // The actual payload to be signed
	}

	resp, err := iamService.Projects.ServiceAccounts.SignJwt(serviceAccountResourceName, signJwtReq).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	if resp == nil || resp.SignedJwt == "" {
		return "", fmt.Errorf("received empty or invalid signed JWT from API")
	}

	return resp.SignedJwt, nil
}

func getTLSCertificatesFromURL(address string) (*tls.Config, error) {
	conn, err := tls.Dial("tcp", address, &tls.Config{})
	if err != nil {
		return nil, fmt.Errorf("error connecting to %s: %v", address, err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates found for %s", address)
	}

	cert := certs[0]

	certPool := x509.NewCertPool()
	certPool.AddCert(cert)

	tlsConfig := &tls.Config{
		RootCAs: certPool,
	}

	return tlsConfig, nil
}

type BuildClientOpts struct {
	// the base of the Url that the grpc service is behind. For example,
	// "my-service.my-company.com". Do not prefix with protocol, do not suffix
	// with port.
	Url string

	// The Service account used to go through the api. The auth JWT is minted
	// using this user as the email. Should have "iap.httpsResourceAccessor"
	// permissions
	ServiceAccount string
}

func BuildClient(ctx context.Context, opts BuildClientOpts) (*grpc.ClientConn, context.Context, error) {
	creds, err := getTLSCertificatesFromURL(fmt.Sprintf("%s:%d", opts.Url, tlsPort))
	if err != nil {
		return nil, nil, fmt.Errorf("err getting tls cert from url %s: %w", opts.Url, err)
	}

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	clientOpts := []client.ConnectOption{
		// always insecure. This is because the lb does tls termination for us.
		client.DialOpts(dialOpts...),

		// we need to use websocket. this allows streaming, and also stops grpc
		// from re-using http conncetions requests that may no longer be correct.
		client.UseWebSocket(true),

		// never use http2.
		client.ForceDowngrade(true),
	}

	token, err := signJWTWithGcloudDefaultCredentials(
		opts.ServiceAccount,
		// wildcard api routes do not exist yet. but I have it on good authority
		// that they will shortly.
		fmt.Sprintf("https://%s/*", opts.Url),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("err minting oauth token: %w", err)
	}

	// we have to use metadata here. This is kind of a hack, but not really.
	// Using the actual authorization types from GRPC means we would have to
	// use some sort of TLS for our GRPCs. We actually do use TLS, just at a
	// layer above, so we're not violating any principles (we're not passing
	// auth credentials in plaintext) but GRPC doesn't know that.
	md := metadata.New(map[string]string{"authorization": fmt.Sprintf("Bearer %s", token)})
	newCtx := metadata.NewOutgoingContext(ctx, md)

	cc, err := client.ConnectViaProxy(
		ctx,
		opts.Url,
		creds,
		clientOpts...,
	)
	if err != nil {
		return nil, nil, err
	}

	return cc, newCtx, nil
}
