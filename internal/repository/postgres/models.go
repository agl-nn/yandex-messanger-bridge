package postgres

// Здесь могут быть дополнительные модели для сложных запросов
// Но основные модели уже определены в domain

// IntegrationStats - статистика по интеграции
type IntegrationStats struct {
	TotalDeliveries int     `db:"total_deliveries"`
	Successful      int     `db:"successful"`
	Failed          int     `db:"failed"`
	LastDelivery    *string `db:"last_delivery"`
}

// DashboardStats - статистика для дашборда
type DashboardStats struct {
	TotalIntegrations  int     `db:"total_integrations"`
	ActiveIntegrations int     `db:"active_integrations"`
	TotalDeliveries    int     `db:"total_deliveries"`
	SuccessRate        float64 `db:"success_rate"`
}
