package cmd

// APIM REST request/response structs

// A few details we need from the 'get instance' response
type apimDetails struct {
	Properties struct {
		MgmtURL                string `json:"managementApiURL"`
		PortalURL              string `json:"developerPortalURL"`
		HostnameConfigurations []struct {
			Type     string
			Hostname string
		}
	}
}

// request body for the ApiManagementUserToken request
type apimTokenRequestProperties struct {
	KeyType string `json:"keyType"`
	Expiry  string `json:"expiry"`
}
type apimTokenRequest struct {
	Propties apimTokenRequestProperties `json:"properties"`
}

// response from ApiManagementUserToken request
type apimTokenRequestResponse struct {
	Value string `json:"value"`
}

// response from the listSecrets request
type apimListSecretsResponse struct {
	URL string `json:"containerSasURL"`
}

// response from contentTypes portal request
type apimPortalContentTypesResponse struct {
	Value []apimPortalContentType `json:"value"`
}

// we only need the Id for now
type apimPortalContentType struct {
	ID string `json:"id"`
}

// response from the contentItems portal reqiest
type apimPortalContentItemsResponse struct {
	Value []interface{} `json:"value"`
}

// response from the contentItems portal reqiest but as a map
type apimPortalContentItemsResponseMap struct {
	Value []map[string]interface{} `json:"value"`
}
