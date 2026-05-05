package models

type User struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type FinancialState struct {
	HasCreditLine           bool  `json:"hasCreditLine"`
	AvailableWorkingCapital int64 `json:"availableWorkingCapital"`
	TaxDebts                bool  `json:"taxDebts"`
}

type Logistics struct {
	OwnTransport     bool `json:"ownTransport"`
	PartnerLogistics bool `json:"partnerLogistics"`
	Warehouse        bool `json:"warehouse"`
}

type Company struct {
	ID                 int64          `json:"id"`
	OwnerID            int64          `json:"ownerId"`
	CompanyName        string         `json:"companyName"`
	INN                string         `json:"inn"`
	OGRN               string         `json:"ogrn"`
	Industry           string         `json:"industry"`
	ExperienceYears    int            `json:"experienceYears"`
	Employees          int            `json:"employees"`
	AnnualRevenue      int64          `json:"annualRevenue"`
	RegionsOfOperation []string       `json:"regionsOfOperation"`
	CompletedContracts []string       `json:"completedContracts"`
	FinancialState     FinancialState `json:"financialState"`
	Logistics          Logistics      `json:"logistics"`
	Certifications     []string       `json:"certifications"`
	KnownLimitations   []string       `json:"knownLimitations"`
	CreatedAt          string         `json:"createdAt"`
	UpdatedAt          string         `json:"updatedAt"`
}

type Tender struct {
	ID                       int64    `json:"id"`
	ExternalTenderID         string   `json:"externalTenderId"`
	PurchaseSubject          string   `json:"purchaseSubject"`
	Region                   string   `json:"region"`
	NMCK                     int64    `json:"nmck"`
	SecurityBid              int64    `json:"securityBid"`
	SecurityContract         int64    `json:"securityContract"`
	ContractGuaranteePercent int      `json:"contractGuaranteePercent"`
	DeliveryPlace            string   `json:"deliveryPlace"`
	DeliveryPeriod           string   `json:"deliveryPeriod"`
	PaymentTerms             string   `json:"paymentTerms"`
	ProcurementMethod        string   `json:"procurementMethod"`
	TenderSummary            []string `json:"tenderSummary"`
	AllRequirements          []string `json:"allRequirements"`
	KeyRequirements          []string `json:"keyRequirements"`
	SourceMessageID          string   `json:"sourceMessageId"`
	CreatedAt                string   `json:"createdAt"`
}

type Risk struct {
	Name   string   `json:"name"`
	Level  string   `json:"level"`
	Basis  []string `json:"basis"`
	Impact string   `json:"impact"`
}

type CompanyFit struct {
	Status        string   `json:"status"`
	Details       []string `json:"details"`
	Label         string   `json:"label"`
	StatusComment string   `json:"statusComment"`
}

type LossEstimate struct {
	Status  string   `json:"status"`
	Label   string   `json:"label"`
	Details []string `json:"details"`
}

type FinalDecision struct {
	Status          string   `json:"status"`
	Details         []string `json:"details"`
	Label           string   `json:"label"`
	DecisionComment string   `json:"decisionComment"`
}

type AnalysisResult struct {
	ID                int64         `json:"id"`
	AnalysisRequestID int64         `json:"analysisRequestId"`
	Summary           string        `json:"summary"`
	CompanyFit        CompanyFit    `json:"companyFit"`
	Risks             []Risk        `json:"risks"`
	Pitfalls          []string      `json:"pitfalls"`
	Recommendations   []string      `json:"recommendations"`
	LossEstimate      LossEstimate  `json:"lossEstimate"`
	FinalDecision     FinalDecision `json:"finalDecision"`
	RawJSON           string        `json:"rawJson"`
	CreatedAt         string        `json:"createdAt"`
}

type AnalysisRequest struct {
	ID                 int64  `json:"id"`
	TenderID           int64  `json:"tenderId"`
	CompanyID          int64  `json:"companyId"`
	UserID             int64  `json:"userId"`
	Status             string `json:"status"`
	ResultID           int64  `json:"resultId,omitempty"`
	FinalDecisionLabel string `json:"finalDecisionLabel,omitempty"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type IntegrationMessage struct {
	ID                int64  `json:"id"`
	ExternalMessageID string `json:"externalMessageId"`
	EventType         string `json:"eventType"`
	ProcessingStatus  string `json:"processingStatus"`
	Payload           string `json:"payload"`
	Description       string `json:"description"`
	ErrorText         string `json:"errorText,omitempty"`
	ReceivedAt        string `json:"receivedAt"`
}

type DashboardSummary struct {
	Totals struct {
		Tenders          int `json:"tenders"`
		Companies        int `json:"companies"`
		AnalysisRequests int `json:"analysisRequests"`
	} `json:"totals"`
	RecentAnalysis    []AnalysisRequestListItem `json:"recentAnalysis"`
	IntegrationEvents []IntegrationMessage      `json:"integrationEvents"`
}

type AnalysisRequestListItem struct {
	ID                 int64  `json:"id"`
	TenderSubject      string `json:"tenderSubject"`
	CompanyName        string `json:"companyName"`
	Status             string `json:"status"`
	FinalDecisionLabel string `json:"finalDecisionLabel,omitempty"`
	ResultID           int64  `json:"resultId,omitempty"`
	CreatedAt          string `json:"createdAt"`
}
