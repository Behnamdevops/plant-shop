package order

// V1 shipping fee configuration. These are small, centralized constants
// rather than a carrier integration or a database-backed pricing table —
// intentionally minimal for V1. Fees are in the same minor-unit currency as
// product prices/order totals (see products.price in AGENTS.md).
//
// To change fees for local development or to tune V1 pricing, edit the
// values below. There is no admin UI or config file for these yet; a real
// pricing model (e.g. per-carrier rates, free-shipping thresholds, weight
// based pricing) is out of scope for V1.
const (
	// ShippingFeeStandard is the flat fee (minor units) charged for
	// "standard" shipping.
	ShippingFeeStandard int64 = 500 // e.g. $5.00 if minor unit = cents

	// ShippingFeeExpress is the flat fee (minor units) charged for
	// "express" shipping.
	ShippingFeeExpress int64 = 1500 // e.g. $15.00 if minor unit = cents
)

// ShippingFeeFor returns the server-side shipping fee for a given shipping
// method. The frontend never supplies a fee directly — it only selects a
// method, and the fee actually charged is always looked up here.
//
// Returns false if method is not a supported V1 shipping method; callers
// must validate the method (e.g. via IsValidShippingMethod /
// CheckoutInput.Validate) before checkout and treat this case as a bug if
// it's ever reached during an actual charge.
func ShippingFeeFor(method string) (int64, bool) {
	switch method {
	case ShippingMethodStandard:
		return ShippingFeeStandard, true
	case ShippingMethodExpress:
		return ShippingFeeExpress, true
	default:
		return 0, false
	}
}
