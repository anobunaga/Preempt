import { useState, useEffect } from 'react';
import './App.css';
import type { HealthResponse, MetricsResponse, AnomaliesResponse, AlarmSuggestionsResponse, LocationsResponse, Location } from './types';

const API_BASE = '/api';

function App() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [metrics, setMetrics] = useState<MetricsResponse | null>(null);
  const [anomalies, setAnomalies] = useState<AnomaliesResponse | null>(null);
  const [suggestions, setSuggestions] = useState<AlarmSuggestionsResponse | null>(null);
  const [locations, setLocations] = useState<Location[]>([]);
  const [selectedLocation, setSelectedLocation] = useState<Location | null>(null);
  const [metricType, setMetricType] = useState('temperature_2m');
  const [hours, setHours] = useState(24);

  // Fetch locations on component mount
  useEffect(() => {
    fetchLocations();
  }, []);

  const fetchLocations = async () => {
    try {
      const res = await fetch(`${API_BASE}/locations`);
      if (!res.ok) {
        console.error('API request failed:', res.status, res.statusText);
        const text = await res.text();
        console.error('Error response:', text);
        return;
      }
      const data: LocationsResponse = await res.json();
      console.log('Locations response:', data);
      console.log('Locations array:', data.locations);
      console.log('First location:', data.locations?.[0]);
      
      if (!data.locations || !Array.isArray(data.locations)) {
        console.error('Invalid locations data:', data);
        return;
      }
      
      setLocations(data.locations);
      // Set first location as default if available
      if (data.locations.length > 0) {
        setSelectedLocation(data.locations[0]);
        console.log('Selected first location:', data.locations[0]);
      } else {
        console.warn('No locations returned from API');
      }
    } catch (err) {
      console.error('Failed to fetch locations:', err);
    }
  };

  const getLocationString = (location: Location | null): string => {
    if (!location || !location.name) {
      return 'Unknown';
    }
    return location.name;
  };

  const fetchHealth = async () => {
    try {
      const res = await fetch(`${API_BASE}/health`);
      const data: HealthResponse = await res.json();
      setHealth(data);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchMetrics = async () => {
    if (!selectedLocation) {
      alert('Please select a location first');
      return;
    }
    try {
      const locationStr = getLocationString(selectedLocation);
      const res = await fetch(`${API_BASE}/metrics?location=${locationStr}&type=${metricType}&hours=${hours}`);
      const data: MetricsResponse = await res.json();
      setMetrics(data);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchAnomalies = async () => {
    if (!selectedLocation) {
      alert('Please select a location first');
      return;
    }
    try {
      const locationStr = getLocationString(selectedLocation);
      const res = await fetch(`${API_BASE}/anomalies?location=${locationStr}&limit=20`);
      const data: AnomaliesResponse = await res.json();
      setAnomalies(data);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchSuggestions = async () => {
    if (!selectedLocation) {
      alert('Please select a location first');
      return;
    }
    try {
      const locationStr = getLocationString(selectedLocation);
      const res = await fetch(`${API_BASE}/alarm-suggestions?location=${locationStr}&limit=15`);
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
        <h2>Location Selection</h2>
        {locations.length > 0 ? (
          <>
            <select 
              value={selectedLocation ? locations.indexOf(selectedLocation) : 0} 
              onChange={e => setSelectedLocation(locations[parseInt(e.target.value)])}
            >
              {locations.map((loc, idx) => (
                <option key={idx} value={idx}>
                  {loc?.name || 'Unknown'}
                </option>
              ))}
            </select>
            {selectedLocation && (
              <p><strong>Selected:</strong> {selectedLocation.name}</p>
            )}
          </>
        ) : (
          <p>Loading locations...</p>
        )}
      </section>

      <section>
        <h2>Health Check</h2>
        <button onClick={fetchHealth}>Check Health</button>
        {health && <pre>{JSON.stringify(health, null, 2)}</pre>}
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
        <button onClick={fetchMetrics} disabled={!selectedLocation}>Fetch Metrics</button>
        {metrics && <pre>{JSON.stringify(metrics, null, 2)}</pre>}
      </section>

      <section>
        <h2>Anomalies</h2>
        <button onClick={fetchAnomalies} disabled={!selectedLocation}>Fetch Anomalies</button>
        {anomalies && <pre>{JSON.stringify(anomalies, null, 2)}</pre>}
      </section>

      <section>
        <h2>Alarm Suggestions</h2>
        <button onClick={fetchSuggestions} disabled={!selectedLocation}>Fetch Suggestions</button>
        {suggestions && <pre>{JSON.stringify(suggestions, null, 2)}</pre>}
      </section>
    </div>
  );
}

export default App;