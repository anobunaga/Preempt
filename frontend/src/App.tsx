import { useState } from 'react';
import './App.css';
import type { HealthResponse, FetchResponse, MetricsResponse, AnomaliesResponse, AlarmSuggestionsResponse, FetchRequest } from './types';

const API_BASE = '/api';

function App() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [currentWeather, setCurrentWeather] = useState<FetchResponse | null>(null);
  const [metrics, setMetrics] = useState<MetricsResponse | null>(null);
  const [anomalies, setAnomalies] = useState<AnomaliesResponse | null>(null);
  const [suggestions, setSuggestions] = useState<AlarmSuggestionsResponse | null>(null);
  const [lat, setLat] = useState(37.7749);
  const [lon, setLon] = useState(-122.4194);
  const [metricType, setMetricType] = useState('temperature_2m');
  const [hours, setHours] = useState(24);
   const [closestCity, setClosestCity] = useState<string>('');

  const fetchHealth = async () => {
    try {
      const res = await fetch(`${API_BASE}/health`);
      const data: HealthResponse = await res.json();
      setHealth(data);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchClosestCity = async (latitude: number, longitude: number) => {
    try {
      const res = await fetch(`https://nominatim.openstreetmap.org/reverse?format=json&lat=${latitude}&lon=${longitude}&zoom=10&addressdetails=1`);
      const data = await res.json();
      // Extract city from the response (prioritize city, then town, etc.)
      const city = data.address?.city || data.address?.town || data.address?.village || 'Unknown location';
      setClosestCity(city);
    } catch (err) {
      console.error('Geocoding error:', err);
      setClosestCity('Unable to determine city');
    }
  };

  const fetchCurrentWeather = async () => {
    try {
      await fetchClosestCity(lat, lon);

      const req: FetchRequest = { latitude: lat, longitude: lon };
      const res = await fetch(`${API_BASE}/fetch-current-weather`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(req),
      });
      const data: FetchResponse = await res.json();
      setCurrentWeather(data);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchMetrics = async () => {
    try {
      const res = await fetch(`${API_BASE}/metrics?type=${metricType}&hours=${hours}`);
      const data: MetricsResponse = await res.json();
      setMetrics(data);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchAnomalies = async () => {
    try {
      const res = await fetch(`${API_BASE}/anomalies?limit=100`);
      const data: AnomaliesResponse = await res.json();
      setAnomalies(data);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchSuggestions = async () => {
    try {
      const res = await fetch(`${API_BASE}/alarm-suggestions?limit=50`);
      const data: AlarmSuggestionsResponse = await res.json();
      setSuggestions(data);
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <div className="App">
      <h1>Preempt</h1>
      
      <section>
        <h2>Health Check</h2>
        <button onClick={fetchHealth}>Check Health</button>
        {health && <pre>{JSON.stringify(health, null, 2)}</pre>}
      </section>

      <section>
        <h2>Fetch Current Weather</h2>
        <input type="number" value={lat} onChange={e => setLat(parseFloat(e.target.value))} placeholder="Latitude" />
        <input type="number" value={lon} onChange={e => setLon(parseFloat(e.target.value))} placeholder="Longitude" />
        <button onClick={fetchCurrentWeather}>Fetch</button>
        {closestCity && <p><strong>Closest City:</strong> {closestCity}</p>}
        {currentWeather && <pre>{JSON.stringify(currentWeather, null, 2)}</pre>}
      </section>

      <section>
        <h2>Metrics</h2>
        <select value={metricType} onChange={e => setMetricType(e.target.value)}>
          <option value="temperature_2m">Temperature</option>
          <option value="relative_humidity_2m">Humidity</option>
          <option value="precipitation">Precipitation</option>
          <option value="wind_speed_10m">Wind Speed</option>
        </select>
        <input type="number" value={hours} onChange={e => setHours(parseInt(e.target.value))} placeholder="Hours" />
        <button onClick={fetchMetrics}>Fetch Metrics</button>
        {metrics && <pre>{JSON.stringify(metrics, null, 2)}</pre>}
      </section>

      <section>
        <h2>Anomalies</h2>
        <button onClick={fetchAnomalies}>Fetch Anomalies</button>
        {anomalies && <pre>{JSON.stringify(anomalies, null, 2)}</pre>}
      </section>

      <section>
        <h2>Alarm Suggestions</h2>
        <button onClick={fetchSuggestions}>Fetch Suggestions</button>
        {suggestions && <pre>{JSON.stringify(suggestions, null, 2)}</pre>}
      </section>
    </div>
  );
}

export default App;
