// Types for the Preempt API

export interface Location {
  id?: number;
  name: string;
  latitude: number;
  longitude: number;
}

export interface LocationsResponse {
  locations: Location[];
  count?: number;
}

export interface HealthResponse {
  status: string;
  time: string;
}

export interface FetchRequest {
  latitude: number;
  longitude: number;
}

export interface FetchResponse {
  status: string;
  anomalies: number;
  timestamp: string;
  forecast: any;
}

export interface CurrentWeather {
  temperature_2m: number;
  relative_humidity_2m: number;
  precipitation: number;
  weather_code: number;
  wind_speed_10m: number;
  wind_direction_10m: number;
  // Add other fields as per monitored fields
}

export interface Metric {
  timestamp: string;
  field: string;
  value: number;
}

export interface MetricsResponse {
  location?: string;
  hours?: number;
  metric_type?: string;
  count?: number;
  data?: any[];
  metrics?: { [field: string]: { count: number; data: Metric[] } };
}

export interface Anomaly {
  id: number;
  timestamp: string;
  field: string;
  value: number;
  expected: number;
  deviation: number;
}

export interface AnomaliesResponse {
  location?: string;
  count: number;
  anomalies: any[];
}

export interface AlarmSuggestion {
  id: number;
  timestamp: string;
  suggestion: string;
  severity: string;
}

export interface AlarmSuggestionsResponse {
  location?: string;
  count: number;
  suggestions: any[];
}