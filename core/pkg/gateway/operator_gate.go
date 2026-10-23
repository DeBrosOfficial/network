package gateway

import (
	"net/http"

	"github.com/Orama-Network/orama/core/pkg/gateway/handlers/operator"
	"go.uber.org/zap"
)

// operatorCaller reports the wallet of the caller and whether it is on the
// cluster's operator list. Only a wallet session names a wallet
// (operator.WalletFromRequest): a key, a token exchanged for one and an app's
// workload token name none, so they are never operators. An error means the
// list could not be read, which is not an answer either way.
func (g *Gateway) operatorCaller(r *http.Request) (wallet string, isOperator bool, err error) {
	wallet = operator.WalletFromRequest(r)
	if wallet == "" {
		return "", false, nil
	}
	isOperator, err = operator.IsOperator(r.Context(), g.ormClient, wallet)
	return wallet, isOperator, err
}

// requireOperatorCaller is the scope gate of an operator route
// (routepolicy.Policy.Operator): it admits a wallet session on the cluster's
// operator list and refuses everyone else, whatever they hold in a namespace.
//
// It reports whether the request may continue; a false return means the
// response has been written.
func (g *Gateway) requireOperatorCaller(w http.ResponseWriter, r *http.Request) bool {
	wallet, isOperator, err := g.operatorCaller(r)
	if err != nil {
		// Not knowing whether someone is an operator is not permission to
		// treat them as one.
		g.logger.ComponentError("gateway", "could not read the operator list", zap.Error(err))
		writeError(w, http.StatusServiceUnavailable,
			"cannot verify operator status right now; the registry did not answer")
		return false
	}
	if isOperator {
		return true
	}
	g.logger.ComponentWarn("gateway", "refused a non-operator on an operator route",
		zap.String("wallet", wallet), zap.String("path", r.URL.Path))
	if wallet == "" {
		forbidden(w, CodeOperatorRequired,
			"this is a cluster operator's route, and only a wallet session can be an operator's: "+
				"an API key, a token exchanged for one or an app's token never is, whatever it holds in its namespace", nil)
		return false
	}
	forbidden(w, CodeOperatorRequired,
		"wallet "+wallet+" is not an operator of this cluster; a role in a namespace, owner included, is not an operator's", nil)
	return false
}
