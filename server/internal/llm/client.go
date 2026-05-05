package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"tender/server/internal/models"
)

type Analyzer struct {
	BaseURL string
	Client  *http.Client
}

type InputPayload struct {
	BrokerPayload  models.Tender  `json:"broker_payload"`
	CompanyProfile models.Company `json:"company_profile"`
	MissingData    []string       `json:"missing_data"`
	TaskType       string         `json:"task_type"`
}

func New(baseURL string) *Analyzer {
	return &Analyzer{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: 300 * time.Second},
	}
}

func (a *Analyzer) Analyze(tender models.Tender, company models.Company) (models.AnalysisResult, error) {
	if a.BaseURL != "" {
		result, err := a.callExternal(tender, company)
		if err == nil {
			log.Println("LLM: external analysis used successfully")
			return result, nil
		}

		log.Printf("LLM ERROR: external analysis failed, fallback will be used: %v", err)
	}

	log.Println("LLM: fallback analysis used")
	return a.fallback(tender, company), nil
}

func (a *Analyzer) callExternal(tender models.Tender, company models.Company) (models.AnalysisResult, error) {
	payload := map[string]any{
		"instruction": "Ты — тендерный риск-менеджер. Верни результат строго в JSON.",
		"input": InputPayload{
			BrokerPayload:  tender,
			CompanyProfile: company,
			MissingData: []string{
				"Не подтвержден полный ассортимент по всем позициям.",
				"Не подтвержден резерв логистики на пиковые периоды.",
			},
			TaskType: "tender_company_analysis",
		},
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return models.AnalysisResult{}, err
	}

	log.Printf("LLM REQUEST URL: %s", a.BaseURL+"/analyze")
	log.Printf("LLM REQUEST BODY:\n%s", string(body))

	req, err := http.NewRequest(http.MethodPost, a.BaseURL+"/analyze", bytes.NewReader(body))
	if err != nil {
		return models.AnalysisResult{}, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.Client.Do(req)
	if err != nil {
		return models.AnalysisResult{}, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.AnalysisResult{}, err
	}

	log.Printf("LLM RESPONSE STATUS: %s", resp.Status)
	log.Printf("LLM RESPONSE BODY:\n%s", string(respBytes))

	if resp.StatusCode >= 300 {
		return models.AnalysisResult{}, fmt.Errorf("llm service returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(respBytes, &raw); err != nil {
		return models.AnalysisResult{}, err
	}

	var result models.AnalysisResult
	if v, ok := raw["output"]; ok {
		if err := json.Unmarshal(v, &result); err != nil {
			return models.AnalysisResult{}, err
		}
	} else {
		encoded, _ := json.Marshal(raw)
		if err := json.Unmarshal(encoded, &result); err != nil {
			return models.AnalysisResult{}, err
		}
	}

	encoded, _ := json.MarshalIndent(result, "", "  ")
	result.RawJSON = string(encoded)

	return result, nil
}

func (a *Analyzer) fallback(tender models.Tender, company models.Company) models.AnalysisResult {
	fitLabel := "conditional_fit"
	fitStatus := "условно соответствует"
	fitComment := "требуется проверка ограничений компании и условий исполнения"
	if company.ExperienceYears >= 3 && len(company.CompletedContracts) >= 2 && !company.FinancialState.TaxDebts {
		fitLabel = "fit"
		fitStatus = "соответствует"
		fitComment = "в целом соответствует предмету закупки"
	}

	finalLabel := "conditional_bid"
	finalStatus := "участвовать после дополнительной проверки"
	decisionComment := "целесообразно участвовать после подтверждения ассортимента, логистики и финансовой модели"
	if fitLabel == "fit" && company.FinancialState.AvailableWorkingCapital > tender.SecurityContract {
		finalLabel = "bid"
		finalStatus = "участвовать"
		decisionComment = "условия участия выглядят приемлемыми при текущем профиле компании"
	}
	if company.FinancialState.TaxDebts || company.FinancialState.AvailableWorkingCapital == 0 {
		fitLabel = "not_fit"
		fitStatus = "не соответствует"
		fitComment = "финансовый профиль компании выглядит недостаточным"
		finalLabel = "no_bid"
		finalStatus = "не участвовать"
		decisionComment = "риски участия превышают ожидаемую выгоду"
	}

	result := models.AnalysisResult{
		Summary: fmt.Sprintf("Компания %s проанализирована по закупке «%s». Ключевые риски связаны с логистикой, подтверждением ассортимента и финансовой устойчивостью при исполнении условий контракта.", company.CompanyName, tender.PurchaseSubject),
		CompanyFit: models.CompanyFit{
			Status: fitStatus,
			Details: []string{
				"Профиль деятельности компании сопоставлен с предметом закупки.",
				fmt.Sprintf("Опыт работы: %d лет, завершённых контрактов: %d.", company.ExperienceYears, len(company.CompletedContracts)),
				fmt.Sprintf("Регион присутствия: %s.", strings.Join(company.RegionsOfOperation, ", ")),
			},
			Label:         fitLabel,
			StatusComment: fitComment,
		},
		Risks: []models.Risk{
			{
				Name:  "Риск исполнения поставки",
				Level: "средний",
				Basis: []string{
					"Поставка осуществляется по условиям тендера с фиксированным сроком исполнения.",
					"Необходимо подтвердить наличие товарного запаса и логистического резерва.",
				},
				Impact: "При нарушении сроков поставки возможны штрафы, претензии заказчика и потеря маржи.",
			},
			{
				Name:  "Риск кассового разрыва",
				Level: chooseRiskLevel(company, tender),
				Basis: []string{
					fmt.Sprintf("Доступный оборотный капитал компании: %d.", company.FinancialState.AvailableWorkingCapital),
					fmt.Sprintf("Обеспечение контракта: %d.", tender.SecurityContract),
					fmt.Sprintf("Условия оплаты: %s.", tender.PaymentTerms),
				},
				Impact: "При недостатке оборотных средств компания может столкнуться с кассовым напряжением в период исполнения.",
			},
		},
		Pitfalls: []string{
			"Недостаточная детализация ассортимента и поставок по заявкам может исказить оценку сложности исполнения.",
			"При отсутствии резервной логистики возрастает чувствительность к пиковым нагрузкам и срочным поставкам.",
		},
		Recommendations: []string{
			"Проверить наличие полного ассортимента и поставщиков по всем позициям.",
			"Подтвердить логистический план и резервный сценарий замены товара.",
			"Оценить достаточность оборотного капитала с учетом отсрочки оплаты и обеспечения контракта.",
		},
		LossEstimate: models.LossEstimate{
			Status: "только сценарная оценка",
			Label:  "scenario_only",
			Details: []string{
				"Потери могут выражаться в штрафах, экстренной логистике, замене товара и временном кассовом разрыве.",
				"Для точного расчета необходимы данные о себестоимости, логистике и фактической маржинальности по позициям.",
			},
		},
		FinalDecision: models.FinalDecision{
			Status:          finalStatus,
			Details:         []string{"Итоговое решение сформировано на основе профиля компании и условий закупки."},
			Label:           finalLabel,
			DecisionComment: decisionComment,
		},
	}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	result.RawJSON = string(encoded)
	return result
}

func chooseRiskLevel(company models.Company, tender models.Tender) string {
	if company.FinancialState.AvailableWorkingCapital < tender.SecurityContract {
		return "высокий"
	}
	if company.FinancialState.AvailableWorkingCapital < tender.SecurityContract*2 {
		return "средний"
	}
	return "низкий"
}
