package validator

import v2 "github.com/zoumo/proxypool/pkg/apis/v2"

// Validator ...
type Validator interface {
	// Validate enqueues a new unverified proxy addr, handle
	// it asynchronously
	Validate(add string)

	// Run starts the Validator
	Run()
	// Stop stops the Validator
	Stop()

	// generic validator for proxy
	// Get returns a validated proxy
	Get() *v2.Proxy
	// Add adds a validated proxy
	Add(*v2.Proxy)
	// Delete delete the proxy
	Delete(*v2.Proxy)
	// Count returns the counts of validated proxy
	Count() int
}

// Queue ...
type Queue interface {
	// Enqueue enqueues a new unverified proxy addr
	Enqueue(addr string)
}
