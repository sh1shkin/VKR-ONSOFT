package store

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "strings"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"

    "tender/server/internal/models"
)

type Store struct {
    pool *pgxpool.Pool
}

func Open(databaseURL string) (*Store, error) {
    if strings.TrimSpace(databaseURL) == "" {
        return nil, errors.New("DATABASE_URL is required")
    }
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    cfg, err := pgxpool.ParseConfig(databaseURL)
    if err != nil {
        return nil, err
    }
    cfg.MaxConns = 10

    pool, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil {
        return nil, err
    }

    st := &Store{pool: pool}
    if err := st.ping(ctx); err != nil {
        pool.Close()
        return nil, err
    }
    if err := st.initSchema(ctx); err != nil {
        pool.Close()
        return nil, err
    }
    return st, nil
}

func (s *Store) ping(ctx context.Context) error {
    return s.pool.Ping(ctx)
}

func (s *Store) initSchema(ctx context.Context) error {
    schema := `
CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'analyst',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON users ((lower(email)));

CREATE TABLE IF NOT EXISTS companies (
  id BIGSERIAL PRIMARY KEY,
  owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  company_name TEXT NOT NULL,
  inn TEXT NOT NULL DEFAULT '',
  ogrn TEXT NOT NULL DEFAULT '',
  industry TEXT NOT NULL DEFAULT '',
  experience_years INTEGER NOT NULL DEFAULT 0,
  employees INTEGER NOT NULL DEFAULT 0,
  annual_revenue BIGINT NOT NULL DEFAULT 0,
  regions_of_operation JSONB NOT NULL DEFAULT '[]'::jsonb,
  completed_contracts JSONB NOT NULL DEFAULT '[]'::jsonb,
  financial_state JSONB NOT NULL DEFAULT '{}'::jsonb,
  logistics JSONB NOT NULL DEFAULT '{}'::jsonb,
  certifications JSONB NOT NULL DEFAULT '[]'::jsonb,
  known_limitations JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_companies_owner_id ON companies(owner_id);

CREATE TABLE IF NOT EXISTS tenders (
  id BIGSERIAL PRIMARY KEY,
  external_tender_id TEXT NOT NULL DEFAULT '',
  purchase_subject TEXT NOT NULL,
  region TEXT NOT NULL DEFAULT '',
  nmck BIGINT NOT NULL DEFAULT 0,
  security_bid BIGINT NOT NULL DEFAULT 0,
  security_contract BIGINT NOT NULL DEFAULT 0,
  contract_guarantee_percent INTEGER NOT NULL DEFAULT 0,
  delivery_place TEXT NOT NULL DEFAULT '',
  delivery_period TEXT NOT NULL DEFAULT '',
  payment_terms TEXT NOT NULL DEFAULT '',
  procurement_method TEXT NOT NULL DEFAULT '',
  tender_summary JSONB NOT NULL DEFAULT '[]'::jsonb,
  all_requirements JSONB NOT NULL DEFAULT '[]'::jsonb,
  key_requirements JSONB NOT NULL DEFAULT '[]'::jsonb,
  source_message_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS integration_messages (
  id BIGSERIAL PRIMARY KEY,
  external_message_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  processing_status TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  error_text TEXT NOT NULL DEFAULT '',
  received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS analysis_requests (
  id BIGSERIAL PRIMARY KEY,
  tender_id BIGINT NOT NULL REFERENCES tenders(id) ON DELETE CASCADE,
  company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'created',
  result_id BIGINT NOT NULL DEFAULT 0,
  final_decision_label TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_analysis_requests_user_id ON analysis_requests(user_id);

CREATE TABLE IF NOT EXISTS analysis_results (
  id BIGSERIAL PRIMARY KEY,
  analysis_request_id BIGINT NOT NULL REFERENCES analysis_requests(id) ON DELETE CASCADE,
  summary TEXT NOT NULL DEFAULT '',
  company_fit JSONB NOT NULL DEFAULT '{}'::jsonb,
  risks JSONB NOT NULL DEFAULT '[]'::jsonb,
  pitfalls JSONB NOT NULL DEFAULT '[]'::jsonb,
  recommendations JSONB NOT NULL DEFAULT '[]'::jsonb,
  loss_estimate JSONB NOT NULL DEFAULT '{}'::jsonb,
  final_decision JSONB NOT NULL DEFAULT '{}'::jsonb,
  raw_json TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
    _, err := s.pool.Exec(ctx, schema)
    return err
}

func (s *Store) Close() { s.pool.Close() }

func nowString(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func jsonBytes(v any) []byte {
    b, _ := json.Marshal(v)
    return b
}

func unmarshalJSON[T any](b []byte, dst *T) error {
    if len(b) == 0 {
        return nil
    }
    return json.Unmarshal(b, dst)
}

func (s *Store) CreateUser(user models.User) (models.User, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    row := s.pool.QueryRow(ctx, `
INSERT INTO users (name, email, password_hash, role)
VALUES ($1, lower($2), $3, $4)
RETURNING id, name, email, password_hash, role, created_at, updated_at`,
        user.Name, user.Email, user.PasswordHash, user.Role,
    )
    return scanUser(row)
}

func (s *Store) FindUserByEmail(email string) (models.User, bool) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
SELECT id, name, email, password_hash, role, created_at, updated_at
FROM users WHERE lower(email)=lower($1)`, email)
    user, err := scanUser(row)
    if err != nil {
        return models.User{}, false
    }
    return user, true
}

func (s *Store) GetUser(id int64) (models.User, bool) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
SELECT id, name, email, password_hash, role, created_at, updated_at
FROM users WHERE id=$1`, id)
    user, err := scanUser(row)
    if err != nil {
        return models.User{}, false
    }
    return user, true
}

func (s *Store) UpdateUser(id int64, updateFn func(*models.User) error) (models.User, error) {
    user, ok := s.GetUser(id)
    if !ok {
        return models.User{}, errors.New("user not found")
    }
    if err := updateFn(&user); err != nil {
        return models.User{}, err
    }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
UPDATE users
SET name=$2, email=lower($3), password_hash=$4, role=$5, updated_at=NOW()
WHERE id=$1
RETURNING id, name, email, password_hash, role, created_at, updated_at`,
        id, user.Name, user.Email, user.PasswordHash, user.Role,
    )
    return scanUser(row)
}

func (s *Store) ListCompanies(ownerID int64, search string) []models.Company {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    q := `
SELECT id, owner_id, company_name, inn, ogrn, industry, experience_years, employees, annual_revenue,
       regions_of_operation, completed_contracts, financial_state, logistics, certifications,
       known_limitations, created_at, updated_at
FROM companies
WHERE owner_id=$1`
    args := []any{ownerID}
    if strings.TrimSpace(search) != "" {
        q += ` AND (LOWER(company_name) LIKE LOWER($2) OR LOWER(industry) LIKE LOWER($2))`
        args = append(args, "%"+strings.TrimSpace(search)+"%")
    }
    q += ` ORDER BY id DESC`
    rows, err := s.pool.Query(ctx, q, args...)
    if err != nil {
        return []models.Company{}
    }
    defer rows.Close()

    out := make([]models.Company, 0)
    for rows.Next() {
        item, err := scanCompany(rows)
        if err == nil {
            out = append(out, item)
        }
    }
    return out
}

func (s *Store) CreateCompany(company models.Company) (models.Company, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
INSERT INTO companies (
  owner_id, company_name, inn, ogrn, industry, experience_years, employees, annual_revenue,
  regions_of_operation, completed_contracts, financial_state, logistics, certifications, known_limitations
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11::jsonb,$12::jsonb,$13::jsonb,$14::jsonb)
RETURNING id, owner_id, company_name, inn, ogrn, industry, experience_years, employees, annual_revenue,
          regions_of_operation, completed_contracts, financial_state, logistics, certifications,
          known_limitations, created_at, updated_at`,
        company.OwnerID, company.CompanyName, company.INN, company.OGRN, company.Industry,
        company.ExperienceYears, company.Employees, company.AnnualRevenue,
        jsonBytes(company.RegionsOfOperation), jsonBytes(company.CompletedContracts),
        jsonBytes(company.FinancialState), jsonBytes(company.Logistics), jsonBytes(company.Certifications),
        jsonBytes(company.KnownLimitations),
    )
    return scanCompany(row)
}

func (s *Store) GetCompany(id, ownerID int64) (models.Company, bool) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
SELECT id, owner_id, company_name, inn, ogrn, industry, experience_years, employees, annual_revenue,
       regions_of_operation, completed_contracts, financial_state, logistics, certifications,
       known_limitations, created_at, updated_at
FROM companies WHERE id=$1 AND owner_id=$2`, id, ownerID)
    item, err := scanCompany(row)
    if err != nil {
        return models.Company{}, false
    }
    return item, true
}

func (s *Store) UpdateCompany(id, ownerID int64, company models.Company) (models.Company, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
UPDATE companies SET
  company_name=$3, inn=$4, ogrn=$5, industry=$6, experience_years=$7, employees=$8, annual_revenue=$9,
  regions_of_operation=$10::jsonb, completed_contracts=$11::jsonb, financial_state=$12::jsonb,
  logistics=$13::jsonb, certifications=$14::jsonb, known_limitations=$15::jsonb, updated_at=NOW()
WHERE id=$1 AND owner_id=$2
RETURNING id, owner_id, company_name, inn, ogrn, industry, experience_years, employees, annual_revenue,
          regions_of_operation, completed_contracts, financial_state, logistics, certifications,
          known_limitations, created_at, updated_at`,
        id, ownerID, company.CompanyName, company.INN, company.OGRN, company.Industry,
        company.ExperienceYears, company.Employees, company.AnnualRevenue,
        jsonBytes(company.RegionsOfOperation), jsonBytes(company.CompletedContracts),
        jsonBytes(company.FinancialState), jsonBytes(company.Logistics), jsonBytes(company.Certifications),
        jsonBytes(company.KnownLimitations),
    )
    return scanCompany(row)
}

func (s *Store) DeleteCompany(id, ownerID int64) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    cmd, err := s.pool.Exec(ctx, `DELETE FROM companies WHERE id=$1 AND owner_id=$2`, id, ownerID)
    if err != nil {
        return err
    }
    if cmd.RowsAffected() == 0 {
        return errors.New("company not found")
    }
    return nil
}

func (s *Store) ListTenders(search string) []models.Tender {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    q := `
SELECT id, external_tender_id, purchase_subject, region, nmck, security_bid, security_contract,
       contract_guarantee_percent, delivery_place, delivery_period, payment_terms, procurement_method,
       tender_summary, all_requirements, key_requirements, source_message_id, created_at
FROM tenders`
    args := []any{}
    if strings.TrimSpace(search) != "" {
        q += ` WHERE LOWER(purchase_subject) LIKE LOWER($1)`
        args = append(args, "%"+strings.TrimSpace(search)+"%")
    }
    q += ` ORDER BY id DESC`
    rows, err := s.pool.Query(ctx, q, args...)
    if err != nil {
        return []models.Tender{}
    }
    defer rows.Close()
    out := make([]models.Tender, 0)
    for rows.Next() {
        item, err := scanTender(rows)
        if err == nil {
            out = append(out, item)
        }
    }
    return out
}

func (s *Store) CreateTender(tender models.Tender) (models.Tender, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
INSERT INTO tenders (
  external_tender_id, purchase_subject, region, nmck, security_bid, security_contract,
  contract_guarantee_percent, delivery_place, delivery_period, payment_terms, procurement_method,
  tender_summary, all_requirements, key_requirements, source_message_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14::jsonb,$15)
RETURNING id, external_tender_id, purchase_subject, region, nmck, security_bid, security_contract,
          contract_guarantee_percent, delivery_place, delivery_period, payment_terms, procurement_method,
          tender_summary, all_requirements, key_requirements, source_message_id, created_at`,
        tender.ExternalTenderID, tender.PurchaseSubject, tender.Region, tender.NMCK, tender.SecurityBid,
        tender.SecurityContract, tender.ContractGuaranteePercent, tender.DeliveryPlace, tender.DeliveryPeriod,
        tender.PaymentTerms, tender.ProcurementMethod, jsonBytes(tender.TenderSummary),
        jsonBytes(tender.AllRequirements), jsonBytes(tender.KeyRequirements), tender.SourceMessageID,
    )
    return scanTender(row)
}

func (s *Store) GetTender(id int64) (models.Tender, bool) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
SELECT id, external_tender_id, purchase_subject, region, nmck, security_bid, security_contract,
       contract_guarantee_percent, delivery_place, delivery_period, payment_terms, procurement_method,
       tender_summary, all_requirements, key_requirements, source_message_id, created_at
FROM tenders WHERE id=$1`, id)
    item, err := scanTender(row)
    if err != nil {
        return models.Tender{}, false
    }
    return item, true
}

func (s *Store) ListIntegrationMessages() []models.IntegrationMessage {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    rows, err := s.pool.Query(ctx, `
SELECT id, external_message_id, event_type, processing_status, payload, description, error_text, received_at
FROM integration_messages ORDER BY id DESC`)
    if err != nil {
        return []models.IntegrationMessage{}
    }
    defer rows.Close()
    out := make([]models.IntegrationMessage, 0)
    for rows.Next() {
        item, err := scanIntegrationMessage(rows)
        if err == nil {
            out = append(out, item)
        }
    }
    return out
}

func (s *Store) CreateIntegrationMessage(msg models.IntegrationMessage) (models.IntegrationMessage, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
INSERT INTO integration_messages (external_message_id, event_type, processing_status, payload, description, error_text)
VALUES ($1,$2,$3,$4,$5,$6)
RETURNING id, external_message_id, event_type, processing_status, payload, description, error_text, received_at`,
        msg.ExternalMessageID, msg.EventType, msg.ProcessingStatus, msg.Payload, msg.Description, msg.ErrorText,
    )
    return scanIntegrationMessage(row)
}

func (s *Store) CreateAnalysisRequest(req models.AnalysisRequest) (models.AnalysisRequest, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
INSERT INTO analysis_requests (tender_id, company_id, user_id, status, result_id, final_decision_label)
VALUES ($1,$2,$3,$4,$5,$6)
RETURNING id, tender_id, company_id, user_id, status, result_id, final_decision_label, created_at, updated_at`,
        req.TenderID, req.CompanyID, req.UserID, req.Status, req.ResultID, req.FinalDecisionLabel,
    )
    return scanAnalysisRequest(row)
}

func (s *Store) UpdateAnalysisRequest(id int64, updateFn func(*models.AnalysisRequest) error) (models.AnalysisRequest, error) {
    req, ok := s.GetAnalysisRequest(id)
    if !ok {
        return models.AnalysisRequest{}, errors.New("analysis request not found")
    }
    if err := updateFn(&req); err != nil {
        return models.AnalysisRequest{}, err
    }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
UPDATE analysis_requests
SET status=$2, result_id=$3, final_decision_label=$4, updated_at=NOW()
WHERE id=$1
RETURNING id, tender_id, company_id, user_id, status, result_id, final_decision_label, created_at, updated_at`,
        id, req.Status, req.ResultID, req.FinalDecisionLabel,
    )
    return scanAnalysisRequest(row)
}

func (s *Store) GetAnalysisRequest(id int64) (models.AnalysisRequest, bool) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
SELECT id, tender_id, company_id, user_id, status, result_id, final_decision_label, created_at, updated_at
FROM analysis_requests WHERE id=$1`, id)
    item, err := scanAnalysisRequest(row)
    if err != nil {
        return models.AnalysisRequest{}, false
    }
    return item, true
}

func (s *Store) CreateAnalysisResult(result models.AnalysisResult) (models.AnalysisResult, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
INSERT INTO analysis_results (
  analysis_request_id, summary, company_fit, risks, pitfalls, recommendations,
  loss_estimate, final_decision, raw_json
) VALUES ($1,$2,$3::jsonb,$4::jsonb,$5::jsonb,$6::jsonb,$7::jsonb,$8::jsonb,$9)
RETURNING id, analysis_request_id, summary, company_fit, risks, pitfalls, recommendations,
          loss_estimate, final_decision, raw_json, created_at`,
        result.AnalysisRequestID, result.Summary, jsonBytes(result.CompanyFit), jsonBytes(result.Risks),
        jsonBytes(result.Pitfalls), jsonBytes(result.Recommendations), jsonBytes(result.LossEstimate),
        jsonBytes(result.FinalDecision), result.RawJSON,
    )
    return scanAnalysisResult(row)
}

func (s *Store) GetAnalysisResult(id int64) (models.AnalysisResult, bool) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    row := s.pool.QueryRow(ctx, `
SELECT id, analysis_request_id, summary, company_fit, risks, pitfalls, recommendations,
       loss_estimate, final_decision, raw_json, created_at
FROM analysis_results WHERE id=$1`, id)
    item, err := scanAnalysisResult(row)
    if err != nil {
        return models.AnalysisResult{}, false
    }
    return item, true
}

func (s *Store) ListAnalysisRequests(userID int64) []models.AnalysisRequest {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    rows, err := s.pool.Query(ctx, `
SELECT id, tender_id, company_id, user_id, status, result_id, final_decision_label, created_at, updated_at
FROM analysis_requests WHERE user_id=$1 ORDER BY id DESC`, userID)
    if err != nil {
        return []models.AnalysisRequest{}
    }
    defer rows.Close()
    out := make([]models.AnalysisRequest, 0)
    for rows.Next() {
        item, err := scanAnalysisRequest(rows)
        if err == nil {
            out = append(out, item)
        }
    }
    return out
}

func (s *Store) DashboardSummary(userID int64) models.DashboardSummary {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    var out models.DashboardSummary
    _ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tenders`).Scan(&out.Totals.Tenders)
    _ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM companies WHERE owner_id=$1`, userID).Scan(&out.Totals.Companies)
    _ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM analysis_requests WHERE user_id=$1`, userID).Scan(&out.Totals.AnalysisRequests)

    rows, err := s.pool.Query(ctx, `
SELECT ar.id, t.purchase_subject, c.company_name, ar.status, ar.final_decision_label, ar.result_id, ar.created_at
FROM analysis_requests ar
JOIN tenders t ON t.id = ar.tender_id
JOIN companies c ON c.id = ar.company_id
WHERE ar.user_id=$1
ORDER BY ar.id DESC
LIMIT 5`, userID)
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var item models.AnalysisRequestListItem
            var createdAt time.Time
            if err := rows.Scan(&item.ID, &item.TenderSubject, &item.CompanyName, &item.Status, &item.FinalDecisionLabel, &item.ResultID, &createdAt); err == nil {
                item.CreatedAt = nowString(createdAt)
                out.RecentAnalysis = append(out.RecentAnalysis, item)
            }
        }
    }

    irows, err := s.pool.Query(ctx, `
SELECT id, external_message_id, event_type, processing_status, payload, description, error_text, received_at
FROM integration_messages ORDER BY id DESC LIMIT 5`)
    if err == nil {
        defer irows.Close()
        for irows.Next() {
            item, err := scanIntegrationMessage(irows)
            if err == nil {
                out.IntegrationEvents = append(out.IntegrationEvents, item)
            }
        }
    }
    return out
}

func (s *Store) SeedDemoData() error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    var tendersCount, messagesCount int
    if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tenders`).Scan(&tendersCount); err != nil {
        return err
    }
    if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM integration_messages`).Scan(&messagesCount); err != nil {
        return err
    }
    if tendersCount > 0 || messagesCount > 0 {
        return nil
    }

    tx, err := s.pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)

    var msg1ID, msg2ID int64
    var msg1Ext, msg2Ext string
    if err := tx.QueryRow(ctx, `
INSERT INTO integration_messages (external_message_id, event_type, processing_status, description, payload)
VALUES ('broker-msg-1001','tender_received','processed','Поступили структурированные данные по закупке медицинских перчаток','{"source":"module-1"}')
RETURNING id, external_message_id`).Scan(&msg1ID, &msg1Ext); err != nil {
        return err
    }
    if err := tx.QueryRow(ctx, `
INSERT INTO integration_messages (external_message_id, event_type, processing_status, description, payload)
VALUES ('broker-msg-1002','tender_received','processed','Поступили структурированные данные по закупке лабораторных реактивов','{"source":"module-1"}')
RETURNING id, external_message_id`).Scan(&msg2ID, &msg2Ext); err != nil {
        return err
    }

    _, err = tx.Exec(ctx, `
INSERT INTO tenders (
 external_tender_id,purchase_subject,region,nmck,security_bid,security_contract,contract_guarantee_percent,
 delivery_place,delivery_period,payment_terms,procurement_method,tender_summary,all_requirements,key_requirements,source_message_id
) VALUES
('T-001','Поставка одноразовых медицинских перчаток для городской клинической больницы','Воронежская область',4850000,48500,242500,5,
 'г. Воронеж, склад заказчика','С даты заключения контракта по 30 ноября 2026 года, партиями по заявкам заказчика',
 'Оплата в течение 45 рабочих дней после приемки','электронный аукцион',
 '["Поставка медицинских перчаток партиями по заявкам заказчика.","Доставка и разгрузка выполняются за счет поставщика.","Необходимы регистрационные документы и документы о качестве."]'::jsonb,
 '["Опыт поставки медицинских расходных материалов.","Поставка партиями по заявкам заказчика.","Остаточный срок годности не менее 70%.","Готовность к замене некачественного товара."]'::jsonb,
 '["Поставка по заявкам","Документы о качестве","Отсрочка оплаты 45 рабочих дней"]'::jsonb,
 $1
),
('T-002','Поставка лабораторных реактивов для областного диагностического центра','Белгородская область',7600000,76000,380000,5,
 'г. Белгород, склад заказчика','В течение 60 календарных дней с даты подписания контракта',
 'Оплата в течение 30 рабочих дней после приемки','электронный аукцион',
 '["Закупка реактивов для лабораторных исследований.","Требуются подтверждающие документы и соблюдение условий хранения."]'::jsonb,
 '["Подтвержденный опыт поставки лабораторных материалов.","Поддержание температурного режима при транспортировке.","Наличие остаточного срока годности."]'::jsonb,
 '["Документы о качестве","Соблюдение условий хранения","Подтвержденный опыт поставки"]'::jsonb,
 $2
)
`, msg1Ext, msg2Ext)
    if err != nil {
        return err
    }

    return tx.Commit(ctx)
}

type scannable interface {
    Scan(dest ...any) error
}

func scanUser(row scannable) (models.User, error) {
    var user models.User
    var createdAt, updatedAt time.Time
    err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &createdAt, &updatedAt)
    if err != nil {
        return models.User{}, err
    }
    user.CreatedAt = nowString(createdAt)
    user.UpdatedAt = nowString(updatedAt)
    return user, nil
}

func scanCompany(row scannable) (models.Company, error) {
    var item models.Company
    var regions, contracts, financialState, logistics, certifications, limitations []byte
    var createdAt, updatedAt time.Time
    err := row.Scan(
        &item.ID, &item.OwnerID, &item.CompanyName, &item.INN, &item.OGRN, &item.Industry,
        &item.ExperienceYears, &item.Employees, &item.AnnualRevenue,
        &regions, &contracts, &financialState, &logistics, &certifications, &limitations,
        &createdAt, &updatedAt,
    )
    if err != nil {
        return models.Company{}, err
    }
    _ = unmarshalJSON(regions, &item.RegionsOfOperation)
    _ = unmarshalJSON(contracts, &item.CompletedContracts)
    _ = unmarshalJSON(financialState, &item.FinancialState)
    _ = unmarshalJSON(logistics, &item.Logistics)
    _ = unmarshalJSON(certifications, &item.Certifications)
    _ = unmarshalJSON(limitations, &item.KnownLimitations)
    item.CreatedAt = nowString(createdAt)
    item.UpdatedAt = nowString(updatedAt)
    return item, nil
}

func scanTender(row scannable) (models.Tender, error) {
    var item models.Tender
    var summary, allReqs, keyReqs []byte
    var createdAt time.Time
    err := row.Scan(
        &item.ID, &item.ExternalTenderID, &item.PurchaseSubject, &item.Region, &item.NMCK,
        &item.SecurityBid, &item.SecurityContract, &item.ContractGuaranteePercent, &item.DeliveryPlace,
        &item.DeliveryPeriod, &item.PaymentTerms, &item.ProcurementMethod,
        &summary, &allReqs, &keyReqs, &item.SourceMessageID, &createdAt,
    )
    if err != nil {
        return models.Tender{}, err
    }
    _ = unmarshalJSON(summary, &item.TenderSummary)
    _ = unmarshalJSON(allReqs, &item.AllRequirements)
    _ = unmarshalJSON(keyReqs, &item.KeyRequirements)
    item.CreatedAt = nowString(createdAt)
    return item, nil
}

func scanIntegrationMessage(row scannable) (models.IntegrationMessage, error) {
    var item models.IntegrationMessage
    var receivedAt time.Time
    err := row.Scan(&item.ID, &item.ExternalMessageID, &item.EventType, &item.ProcessingStatus, &item.Payload, &item.Description, &item.ErrorText, &receivedAt)
    if err != nil {
        return models.IntegrationMessage{}, err
    }
    item.ReceivedAt = nowString(receivedAt)
    return item, nil
}

func scanAnalysisRequest(row scannable) (models.AnalysisRequest, error) {
    var item models.AnalysisRequest
    var createdAt, updatedAt time.Time
    err := row.Scan(&item.ID, &item.TenderID, &item.CompanyID, &item.UserID, &item.Status, &item.ResultID, &item.FinalDecisionLabel, &createdAt, &updatedAt)
    if err != nil {
        return models.AnalysisRequest{}, err
    }
    item.CreatedAt = nowString(createdAt)
    item.UpdatedAt = nowString(updatedAt)
    return item, nil
}

func scanAnalysisResult(row scannable) (models.AnalysisResult, error) {
    var item models.AnalysisResult
    var companyFit, risks, pitfalls, recommendations, lossEstimate, finalDecision []byte
    var createdAt time.Time
    err := row.Scan(&item.ID, &item.AnalysisRequestID, &item.Summary, &companyFit, &risks, &pitfalls, &recommendations, &lossEstimate, &finalDecision, &item.RawJSON, &createdAt)
    if err != nil {
        return models.AnalysisResult{}, err
    }
    _ = unmarshalJSON(companyFit, &item.CompanyFit)
    _ = unmarshalJSON(risks, &item.Risks)
    _ = unmarshalJSON(pitfalls, &item.Pitfalls)
    _ = unmarshalJSON(recommendations, &item.Recommendations)
    _ = unmarshalJSON(lossEstimate, &item.LossEstimate)
    _ = unmarshalJSON(finalDecision, &item.FinalDecision)
    item.CreatedAt = nowString(createdAt)
    return item, nil
}

func (s *Store) String() string {
    return fmt.Sprintf("postgres store")
}
