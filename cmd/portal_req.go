package cmd

// APIM REST request/response structs

// A few details we need from the 'get instance' response
type apimDetails struct {
	Properties struct {
		MgmtUrl                string `json:"managementApiUrl"`
		PortalUrl              string `json:"developerPortalUrl"`
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
	Value string `json:"value`
}

// response from the listSecrets request
type apimListSecretsResponse struct {
	Url string `json:"containerSasUrl"`
}

// response from contentTypes portal request
type apimPortalContentTypesResponse struct {
	Value []apimPortalContentType `json:"value"`
}

// we only need the Id for now
type apimPortalContentType struct {
	Id string `json:"id"`
}

// response from the contentItems portal reqiest
type apimPortalContentItemsResponse struct {
	Value []map[string]interface{} `json:"value"`
}

// we only need the Id for now
type apimPortalContentItem struct {
	Id string `json:"id"`
}
