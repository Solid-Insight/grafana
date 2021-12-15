package azuremonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/tsdb/azuremonitor/azcredentials"
)

const (
	timeSeries = "time_series"
)

var (
	azlog           = log.New("tsdb.azuremonitor")
	legendKeyFormat = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)
)

func ProvideService(cfg *setting.Cfg, httpClientProvider *httpclient.Provider) *Service {
	proxy := &httpServiceProxy{}
	executors := map[string]azDatasourceExecutor{
		azureMonitor:       &AzureMonitorDatasource{proxy: proxy},
		appInsights:        &ApplicationInsightsDatasource{proxy: proxy},
		azureLogAnalytics:  &AzureLogAnalyticsDatasource{proxy: proxy},
		insightsAnalytics:  &InsightsAnalyticsDatasource{proxy: proxy},
		azureResourceGraph: &AzureResourceGraphDatasource{proxy: proxy},
	}
	im := datasource.NewInstanceManager(NewInstanceSettings(cfg, *httpClientProvider, executors))

	s := &Service{
		Cfg:         cfg,
		im:          im,
		executors:   executors,
		dataMux:     datasource.NewQueryTypeMux(),
		resourceMux: http.NewServeMux(),
	}

	s.registerDataRoutes()
	s.registerResourceRoutes()

	return s
}

type serviceProxy interface {
	Do(rw http.ResponseWriter, req *http.Request, cli *http.Client) http.ResponseWriter
}

type Service struct {
	dataMux     *datasource.QueryTypeMux
	resourceMux *http.ServeMux
	Cfg         *setting.Cfg
	im          instancemgmt.InstanceManager
	executors   map[string]azDatasourceExecutor
}

type azureMonitorSettings struct {
	SubscriptionId               string `json:"subscriptionId"`
	LogAnalyticsDefaultWorkspace string `json:"logAnalyticsDefaultWorkspace"`
	AppInsightsAppId             string `json:"appInsightsAppId"`
}

type datasourceInfo struct {
	Cloud       string
	Credentials azcredentials.AzureCredentials
	Settings    azureMonitorSettings
	Routes      map[string]azRoute
	Services    map[string]datasourceService

	JSONData                map[string]interface{}
	DecryptedSecureJSONData map[string]string
	DatasourceID            int64
	OrgID                   int64
}

type datasourceService struct {
	URL        string
	HTTPClient *http.Client
}

func getDatasourceService(cfg *setting.Cfg, clientProvider httpclient.Provider, dsInfo datasourceInfo, routeName string) (datasourceService, error) {
	route := dsInfo.Routes[routeName]
	client, err := newHTTPClient(route, dsInfo, cfg, clientProvider)
	if err != nil {
		return datasourceService{}, err
	}
	return datasourceService{
		URL:        dsInfo.Routes[routeName].URL,
		HTTPClient: client,
	}, nil
}

func NewInstanceSettings(cfg *setting.Cfg, clientProvider httpclient.Provider, executors map[string]azDatasourceExecutor) datasource.InstanceFactoryFunc {
	return func(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		jsonData, err := simplejson.NewJson(settings.JSONData)
		if err != nil {
			return nil, fmt.Errorf("error reading settings: %w", err)
		}

		jsonDataObj := map[string]interface{}{}
		err = json.Unmarshal(settings.JSONData, &jsonDataObj)
		if err != nil {
			return nil, fmt.Errorf("error reading settings: %w", err)
		}

		azMonitorSettings := azureMonitorSettings{}
		err = json.Unmarshal(settings.JSONData, &azMonitorSettings)
		if err != nil {
			return nil, fmt.Errorf("error reading settings: %w", err)
		}

		cloud, err := getAzureCloud(cfg, jsonData)
		if err != nil {
			return nil, fmt.Errorf("error getting credentials: %w", err)
		}

		credentials, err := getAzureCredentials(cfg, jsonData, settings.DecryptedSecureJSONData)
		if err != nil {
			return nil, fmt.Errorf("error getting credentials: %w", err)
		}

		model := datasourceInfo{
			Cloud:                   cloud,
			Credentials:             credentials,
			Settings:                azMonitorSettings,
			JSONData:                jsonDataObj,
			DecryptedSecureJSONData: settings.DecryptedSecureJSONData,
			DatasourceID:            settings.ID,
			Routes:                  routes[cloud],
			Services:                map[string]datasourceService{},
		}

		for routeName := range executors {
			service, err := getDatasourceService(cfg, clientProvider, model, routeName)
			if err != nil {
				return nil, err
			}
			model.Services[routeName] = service
		}

		return model, nil
	}
}

type azDatasourceExecutor interface {
	executeTimeSeriesQuery(ctx context.Context, originalQueries []backend.DataQuery, dsInfo datasourceInfo, client *http.Client, url string) (*backend.QueryDataResponse, error)
	resourceRequest(rw http.ResponseWriter, req *http.Request, cli *http.Client)
}

func (s *Service) getDataSourceFromPluginReq(req *backend.QueryDataRequest) (datasourceInfo, error) {
	i, err := s.im.Get(req.PluginContext)
	if err != nil {
		return datasourceInfo{}, err
	}
	dsInfo, ok := i.(datasourceInfo)
	if !ok {
		return datasourceInfo{}, fmt.Errorf("unable to convert datasource from service instance")
	}
	dsInfo.OrgID = req.PluginContext.OrgID
	return dsInfo, nil
}

func (s *Service) registerDataRoutes() {
	if s.dataMux == nil {
		s.dataMux = datasource.NewQueryTypeMux()
	}

	for dsType := range s.executors {
		// Make a copy of the string to keep the reference after the iterator
		dst := dsType
		s.dataMux.HandleFunc(dsType, func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			executor := s.executors[dst]
			dsInfo, err := s.getDataSourceFromPluginReq(req)
			if err != nil {
				return nil, err
			}
			service, ok := dsInfo.Services[dst]
			if !ok {
				return nil, fmt.Errorf("missing service for %s", dst)
			}
			return executor.executeTimeSeriesQuery(ctx, req.Queries, dsInfo, service.HTTPClient, service.URL)
		})
	}
}

func (s *Service) registerResourceRoutes() {
	if s.resourceMux == nil {
		s.resourceMux = http.NewServeMux()
	}

	s.resourceMux.HandleFunc("/azuremonitor/", s.resourceHandler(azureMonitor))
	s.resourceMux.HandleFunc("/appinsights/", s.resourceHandler(appInsights))
	s.resourceMux.HandleFunc("/loganalytics/", s.resourceHandler(azureLogAnalytics))
	s.resourceMux.HandleFunc("/resourcegraph/", s.resourceHandler(azureResourceGraph))
}

func (s *Service) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	return s.dataMux.QueryData(ctx, req)
}

func (s *Service) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return httpadapter.New(s.resourceMux).CallResource(ctx, req, sender)
}
