# Speciality Steels Water Management System

## Quick Start
```bash
./water-mgmt
```
Then open http://localhost:8080

## Architecture
- Single Go binary, no external dependencies
- JSON file storage (`data/water.json`)
- REST API + dark-themed web dashboard

## API Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| /api/sites | GET | List all sites |
| /api/meters?site_id= | GET | List meters |
| /api/readings | GET/POST | Read/add meter readings |
| /api/tonnes | GET/POST | Read/add tonnes data |
| /api/dashboard?site_id=&from=&to= | GET | Dashboard data |
| /api/kpis | GET | m³/tonne KPIs per site |
| /api/search?q=&site_id=&water_type=&from=&to= | GET | Search readings |
| /api/report?from=&to= | GET | Generate report |
| /api/seed | POST | Load sample data from xlsx |
| /api/eemon/sync | POST | Configure EEmon integration |
| /api/trend/sync | POST | Configure Trend integration |

## Sites & Meter Configuration
### Rotherham (Manual reads)
- **TCM Feed**: Thrybergh Main Gate, Gate House, TBM Tunnel, Visitors Centre, TBM River, RWTP
- **ACP Dept**: ACP Main Feed, Kress Square, VAC Degasser, ABC RW, ABC Towns Water, AMS Amenities
- **BBR Dept**: BBR, Bike Shed, AOCC
- **Aldwarke Lane**: BOC, Cap Gemini (Left/Right), Test House

### Stocksbridge (EEmon + Manual)
- **Towns - East Bank**: RMS1, RMS2, VIM, BM Meter 16/17, BM Saws 14, Croft Supply
- **River - West Bank**: Water Quench 1&2, Spring Bank
- **Manual**: West Bank Gardens, Gate Meters, EB Trade Effluent, EB Network, EB Tank

### Brinsworth (Trend + Manual)
- **Manual**: Total Towns Water, Grange Lane Reservoir
- **Trend**: Hot Mill River, HRP River, Cold Mill River

### Wednesbury (Manual)
- Main Meter 1 & 2, Production, Amenities

## EEmon Integration
POST /api/eemon/sync with `{"base_url":"...","username":"...","password":"..."}`

## Trend Integration
POST /api/trend/sync with `{"base_url":"...","api_key":"..."}`
