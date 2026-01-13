package proxy

// dashboardHTML contains the complete HTML/CSS/JS for the monitoring dashboard
// Updated with SVG sparklines, donuts, and enhanced modal with tabs
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ollama-auto-ctx Dashboard</title>
    <style>
        :root {
            --bg-primary: #1e293b;
            --bg-secondary: #0f172a;
            --bg-card: #1e293b;
            --bg-hover: #334155;
            --border-color: #334155;
            --text-primary: #f1f5f9;
            --text-secondary: #cbd5e1;
            --text-muted: #94a3b8;
            --accent-blue: #3b82f6;
            --accent-green: #10b981;
            --accent-red: #ef4444;
            --accent-yellow: #f59e0b;
            --accent-orange: #f97316;
            --accent-purple: #8b5cf6;
        }

        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: var(--bg-secondary);
            color: var(--text-primary);
            line-height: 1.6;
        }

        .container { max-width: 1400px; margin: 0 auto; padding: 24px; }

        header {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            padding: 20px 24px;
            border-radius: 8px;
            margin-bottom: 24px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        h1 { color: var(--text-primary); font-size: 24px; font-weight: 600; }

        .header-right { display: flex; align-items: center; gap: 16px; }

        .window-selector {
            display: flex;
            gap: 4px;
            background: var(--bg-secondary);
            padding: 4px;
            border-radius: 6px;
        }

        .window-btn {
            padding: 6px 12px;
            border: none;
            background: transparent;
            color: var(--text-secondary);
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            transition: all 0.2s;
        }

        .window-btn:hover { color: var(--text-primary); }
        .window-btn.active { background: var(--accent-blue); color: white; }

        .health-badge {
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 6px 12px;
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 4px;
            font-size: 14px;
        }

        .health-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: var(--accent-green);
        }

        .health-dot.unhealthy { background: var(--accent-red); }

        /* Summary Cards with Sparklines */
        .summary-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
            gap: 16px;
            margin-bottom: 24px;
        }

        .summary-card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            padding: 20px;
            border-radius: 8px;
            position: relative;
        }

        .summary-card-header {
            display: flex;
            justify-content: space-between;
            align-items: flex-start;
            margin-bottom: 8px;
        }

        .summary-label {
            color: var(--text-secondary);
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            font-weight: 600;
        }

        .summary-value {
            font-size: 28px;
            font-weight: 600;
            color: var(--text-primary);
            font-variant-numeric: tabular-nums;
        }

        .summary-subtext {
            color: var(--text-muted);
            font-size: 13px;
            margin-top: 4px;
        }

        /* Sparkline SVG */
        .sparkline-container {
            height: 40px;
            margin-top: 12px;
        }

        .sparkline {
            width: 100%;
            height: 100%;
        }

        .sparkline-line {
            fill: none;
            stroke: var(--accent-blue);
            stroke-width: 1.5;
            stroke-linecap: round;
            stroke-linejoin: round;
        }

        .sparkline-area {
            fill: url(#sparklineGradient);
            opacity: 0.3;
        }

        /* Donut Chart */
        .donut-container {
            width: 60px;
            height: 60px;
        }

        .donut-ring {
            fill: transparent;
            stroke: var(--border-color);
            stroke-width: 4;
        }

        .donut-segment {
            fill: transparent;
            stroke-width: 4;
            stroke-linecap: round;
            transform-origin: center;
            transform: rotate(-90deg);
        }

        .donut-success { stroke: var(--accent-green); }
        .donut-error { stroke: var(--accent-red); }
        .donut-timeout { stroke: var(--accent-yellow); }

        /* Main Content */
        .main-content {
            display: grid;
            grid-template-columns: 1fr;
            gap: 24px;
        }

        .card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 24px;
        }

        .card-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
        }

        .card-title { font-size: 18px; font-weight: 600; }
        .card-subtitle { font-size: 14px; color: var(--text-muted); margin-top: 4px; }

        /* Table */
        .table-container { overflow-x: auto; }

        table { width: 100%; border-collapse: collapse; }

        thead { background: var(--bg-secondary); }

        th {
            padding: 12px 16px;
            text-align: left;
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            color: var(--text-secondary);
            font-weight: 600;
            border-bottom: 1px solid var(--border-color);
        }

        td {
            padding: 12px 16px;
            border-bottom: 1px solid var(--border-color);
            font-size: 14px;
        }

        tbody tr { cursor: pointer; transition: background 0.2s; }
        tbody tr:hover { background: var(--bg-hover); }

        /* Status Badge */
        .status-badge {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            padding: 4px 10px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 600;
            text-transform: uppercase;
        }

        .status-badge::before {
            content: '';
            width: 6px;
            height: 6px;
            border-radius: 50%;
        }

        .status-success { background: rgba(16, 185, 129, 0.15); color: var(--accent-green); }
        .status-success::before { background: var(--accent-green); }
        .status-error { background: rgba(239, 68, 68, 0.15); color: var(--accent-red); }
        .status-error::before { background: var(--accent-red); }
        .status-timeout { background: rgba(245, 158, 11, 0.15); color: var(--accent-yellow); }
        .status-timeout::before { background: var(--accent-yellow); }
        .status-inflight { background: rgba(59, 130, 246, 0.15); color: var(--accent-blue); }
        .status-inflight::before { background: var(--accent-blue); animation: pulse 1.5s infinite; }

        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.4; }
        }

        /* Modal */
        .modal {
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.7);
            display: flex;
            justify-content: center;
            align-items: center;
            z-index: 1000;
        }

        .modal-content {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            max-width: 900px;
            width: 95%;
            max-height: 90vh;
            overflow: hidden;
            display: flex;
            flex-direction: column;
        }

        .modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 20px 24px;
            border-bottom: 1px solid var(--border-color);
        }

        .modal-header h2 { font-size: 18px; }

        .modal-close {
            background: none;
            border: none;
            color: var(--text-secondary);
            font-size: 24px;
            cursor: pointer;
            padding: 4px 8px;
            border-radius: 4px;
        }

        .modal-close:hover { background: var(--bg-hover); color: var(--text-primary); }

        /* Modal Tabs */
        .modal-tabs {
            display: flex;
            border-bottom: 1px solid var(--border-color);
            background: var(--bg-secondary);
        }

        .modal-tab {
            padding: 12px 20px;
            border: none;
            background: transparent;
            color: var(--text-secondary);
            font-size: 14px;
            cursor: pointer;
            border-bottom: 2px solid transparent;
            margin-bottom: -1px;
        }

        .modal-tab:hover { color: var(--text-primary); }
        .modal-tab.active { color: var(--accent-blue); border-bottom-color: var(--accent-blue); }

        .modal-body {
            padding: 24px;
            overflow-y: auto;
            flex: 1;
        }

        .tab-content { display: none; }
        .tab-content.active { display: block; }

        /* Flow Diagram */
        .flow-diagram {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 24px 0;
            gap: 16px;
        }

        .flow-box {
            flex: 1;
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 16px;
            text-align: center;
        }

        .flow-box-title {
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            color: var(--text-secondary);
            margin-bottom: 12px;
        }

        .flow-arrow {
            color: var(--text-muted);
            font-size: 20px;
        }

        .flow-metric {
            display: flex;
            justify-content: space-between;
            padding: 6px 0;
            font-size: 13px;
        }

        .flow-metric-label { color: var(--text-secondary); }
        .flow-metric-value { color: var(--text-primary); font-weight: 500; font-variant-numeric: tabular-nums; }

        /* Timing Bar */
        .timing-bar-container { margin: 16px 0; }

        .timing-bar-label {
            display: flex;
            justify-content: space-between;
            margin-bottom: 6px;
            font-size: 13px;
        }

        .timing-bar {
            height: 8px;
            background: var(--bg-secondary);
            border-radius: 4px;
            overflow: hidden;
        }

        .timing-bar-fill {
            height: 100%;
            border-radius: 4px;
            transition: width 0.3s;
        }

        /* Detail Grid */
        .detail-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 16px;
        }

        .detail-item {
            padding: 12px;
            background: var(--bg-secondary);
            border-radius: 6px;
        }

        .detail-label {
            font-size: 11px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            color: var(--text-muted);
            margin-bottom: 4px;
        }

        .detail-value {
            font-size: 16px;
            font-weight: 500;
            color: var(--text-primary);
            font-variant-numeric: tabular-nums;
        }

        /* Pagination */
        .pagination {
            display: flex;
            justify-content: center;
            align-items: center;
            gap: 16px;
            margin-top: 16px;
            padding-top: 16px;
            border-top: 1px solid var(--border-color);
        }

        .pagination-btn {
            padding: 8px 16px;
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            color: var(--text-primary);
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
        }

        .pagination-btn:hover:not(:disabled) { background: var(--bg-hover); }
        .pagination-btn:disabled { opacity: 0.5; cursor: not-allowed; }

        .page-info { color: var(--text-secondary); font-size: 14px; }

        /* Loading & Empty States */
        .loading, .empty-state {
            text-align: center;
            padding: 48px 24px;
            color: var(--text-muted);
        }

        .loading::after {
            content: '';
            display: inline-block;
            width: 16px;
            height: 16px;
            border: 2px solid var(--border-color);
            border-radius: 50%;
            border-top-color: var(--accent-blue);
            animation: spin 1s linear infinite;
            margin-left: 8px;
        }

        @keyframes spin { to { transform: rotate(360deg); } }

        /* Responsive */
        @media (max-width: 768px) {
            .container { padding: 16px; }
            header { flex-direction: column; gap: 16px; }
            .summary-grid { grid-template-columns: 1fr; }
            .flow-diagram { flex-direction: column; }
            .flow-arrow { transform: rotate(90deg); }
        }
    </style>
</head>
<body>
    <!-- SVG Gradient Definition -->
    <svg width="0" height="0" style="position: absolute;">
        <defs>
            <linearGradient id="sparklineGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" style="stop-color: var(--accent-blue); stop-opacity: 0.4" />
                <stop offset="100%" style="stop-color: var(--accent-blue); stop-opacity: 0" />
            </linearGradient>
        </defs>
    </svg>

    <div class="container">
        <header>
            <h1>ollama-auto-ctx</h1>
            <div class="header-right">
                <div class="window-selector">
                    <button class="window-btn" data-window="1h">1H</button>
                    <button class="window-btn active" data-window="24h">24H</button>
                    <button class="window-btn" data-window="7d">7D</button>
                </div>
                <div class="health-badge">
                    <div class="health-dot" id="healthDot"></div>
                    <span id="healthText">Checking...</span>
                </div>
            </div>
        </header>

        <!-- Summary Cards -->
        <div class="summary-grid">
            <div class="summary-card">
                <div class="summary-card-header">
                    <div>
                        <div class="summary-label">Total Requests</div>
                        <div class="summary-value" id="totalRequests">-</div>
                        <div class="summary-subtext"><span id="inFlightCount">0</span> in-flight</div>
                    </div>
                    <div class="donut-container" id="statusDonut"></div>
                </div>
                <div class="sparkline-container" id="reqCountSparkline"></div>
            </div>

            <div class="summary-card">
                <div class="summary-card-header">
                    <div>
                        <div class="summary-label">Success Rate</div>
                        <div class="summary-value" id="successRate">-</div>
                        <div class="summary-subtext" id="successRateSub">-</div>
                    </div>
                </div>
                <div class="sparkline-container" id="successSparkline"></div>
            </div>

            <div class="summary-card">
                <div class="summary-card-header">
                    <div>
                        <div class="summary-label">Avg Duration</div>
                        <div class="summary-value" id="avgDuration">-</div>
                        <div class="summary-subtext">P95: <span id="p95Duration">-</span></div>
                    </div>
                </div>
                <div class="sparkline-container" id="durationSparkline"></div>
            </div>

            <div class="summary-card">
                <div class="summary-card-header">
                    <div>
                        <div class="summary-label">Total Tokens</div>
                        <div class="summary-value" id="totalTokens">-</div>
                        <div class="summary-subtext"><span id="totalBytes">-</span> transferred</div>
                    </div>
                </div>
                <div class="sparkline-container" id="tokensSparkline"></div>
            </div>
        </div>

        <!-- Recent Requests -->
        <div class="card">
            <div class="card-header">
                <div>
                    <div class="card-title">Recent Requests</div>
                    <div class="card-subtitle">Click a row for details</div>
                </div>
            </div>
            <div class="table-container">
                <div id="loadingState" class="loading">Loading requests...</div>
                <table id="requestsTable" style="display: none;">
                    <thead>
                        <tr>
                            <th>Time</th>
                            <th>Model</th>
                            <th>Endpoint</th>
                            <th>Duration</th>
                            <th>Bytes</th>
                            <th>Status</th>
                        </tr>
                    </thead>
                    <tbody id="tableBody"></tbody>
                </table>
                <div id="emptyState" class="empty-state" style="display: none;">No requests yet</div>
            </div>
            <div id="pagination" class="pagination" style="display: none;">
                <button id="prevPage" class="pagination-btn" disabled>Previous</button>
                <span id="pageInfo" class="page-info">Page 1</span>
                <button id="nextPage" class="pagination-btn">Next</button>
            </div>
        </div>
    </div>

    <!-- Request Detail Modal -->
    <div id="requestModal" class="modal" style="display: none;">
        <div class="modal-content">
            <div class="modal-header">
                <h2>Request Details</h2>
                <button class="modal-close" onclick="closeModal()">&times;</button>
            </div>
            <div class="modal-tabs">
                <button class="modal-tab active" data-tab="overview">Overview</button>
                <button class="modal-tab" data-tab="timings">Timings</button>
                <button class="modal-tab" data-tab="tokens">Tokens</button>
                <button class="modal-tab" data-tab="bytes">Bytes</button>
            </div>
            <div class="modal-body">
                <div id="tab-overview" class="tab-content active"></div>
                <div id="tab-timings" class="tab-content"></div>
                <div id="tab-tokens" class="tab-content"></div>
                <div id="tab-bytes" class="tab-content"></div>
            </div>
        </div>
    </div>

    <script>
        // State
        let currentWindow = '24h';
        let currentPage = 1;
        const pageSize = 20;
        let overviewData = null;
        let requestsData = [];

        // API base path
        const API = '/autoctx/api/v1';

        // Initialize
        document.addEventListener('DOMContentLoaded', () => {
            fetchOverview();
            fetchRequests();
            fetchHealth();
            setInterval(fetchOverview, 5000);
            setInterval(fetchRequests, 3000);
            setInterval(fetchHealth, 10000);
        });

        // Window selector
        document.querySelectorAll('.window-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                document.querySelectorAll('.window-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                currentWindow = btn.dataset.window;
                fetchOverview();
            });
        });

        // Modal tabs
        document.querySelectorAll('.modal-tab').forEach(tab => {
            tab.addEventListener('click', () => {
                document.querySelectorAll('.modal-tab').forEach(t => t.classList.remove('active'));
                document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
                tab.classList.add('active');
                document.getElementById('tab-' + tab.dataset.tab).classList.add('active');
            });
        });

        // Pagination
        document.getElementById('prevPage').addEventListener('click', () => {
            if (currentPage > 1) { currentPage--; renderRequests(); }
        });
        document.getElementById('nextPage').addEventListener('click', () => {
            currentPage++;
            renderRequests();
        });

        // Close modal on outside click
        document.getElementById('requestModal').addEventListener('click', (e) => {
            if (e.target.id === 'requestModal') closeModal();
        });

        // Fetch functions
        async function fetchOverview() {
            try {
                const res = await fetch(API + '/overview?window=' + currentWindow);
                if (!res.ok) throw new Error('Failed to fetch overview');
                overviewData = await res.json();
                renderOverview();
            } catch (err) {
                console.error('Overview error:', err);
            }
        }

        async function fetchRequests() {
            try {
                const res = await fetch(API + '/requests?limit=100&window=' + currentWindow);
                if (!res.ok) throw new Error('Failed to fetch requests');
                const data = await res.json();
                requestsData = data.requests || [];
                renderRequests();
            } catch (err) {
                console.error('Requests error:', err);
            }
        }

        async function fetchHealth() {
            try {
                const res = await fetch('/healthz/upstream');
                const data = await res.json();
                updateHealth(data.healthy);
            } catch (err) {
                updateHealth(false);
            }
        }

        async function fetchRequestDetail(id) {
            try {
                const res = await fetch(API + '/requests/' + id);
                if (!res.ok) throw new Error('Failed to fetch request');
                return await res.json();
            } catch (err) {
                console.error('Request detail error:', err);
                return null;
            }
        }

        // Render functions
        function renderOverview() {
            if (!overviewData) return;
            const s = overviewData.summary;

            document.getElementById('totalRequests').textContent = formatNumber(s.total_requests);
            document.getElementById('inFlightCount').textContent = s.in_flight || 0;
            document.getElementById('successRate').textContent = (s.success_rate * 100).toFixed(1) + '%';
            document.getElementById('successRateSub').textContent = 
                formatNumber(Math.round(s.total_requests * s.success_rate)) + ' / ' + formatNumber(s.total_requests);
            document.getElementById('avgDuration').textContent = formatDuration(s.avg_duration_ms);
            document.getElementById('p95Duration').textContent = formatDuration(s.p95_duration_ms);
            document.getElementById('totalTokens').textContent = formatNumber(s.total_tokens);
            document.getElementById('totalBytes').textContent = formatBytes(s.total_bytes);

            // Render sparklines
            renderSparkline('reqCountSparkline', overviewData.series.req_count, '--accent-blue');
            renderSparkline('durationSparkline', overviewData.series.duration_p95, '--accent-purple');
            renderSparkline('tokensSparkline', overviewData.series.gen_tok_per_s, '--accent-green');

            // Render status donut
            const errorCount = s.total_requests - Math.round(s.total_requests * s.success_rate);
            renderDonut('statusDonut', s.success_rate, s.timeouts / Math.max(1, s.total_requests));
        }

        function renderSparkline(containerId, data, colorVar) {
            const container = document.getElementById(containerId);
            if (!data || data.length === 0) {
                container.innerHTML = '';
                return;
            }

            const values = data.map(d => d.value);
            const max = Math.max(...values, 1);
            const min = Math.min(...values, 0);
            const range = max - min || 1;

            const width = 100;
            const height = 30;
            const points = values.map((v, i) => {
                const x = (i / (values.length - 1)) * width;
                const y = height - ((v - min) / range) * height;
                return x + ',' + y;
            }).join(' ');

            const areaPoints = points + ' ' + width + ',' + height + ' 0,' + height;

            container.innerHTML = 
                '<svg class="sparkline" viewBox="0 0 ' + width + ' ' + height + '" preserveAspectRatio="none">' +
                '<polygon class="sparkline-area" points="' + areaPoints + '"/>' +
                '<polyline class="sparkline-line" style="stroke: var(' + colorVar + ')" points="' + points + '"/>' +
                '</svg>';
        }

        function renderDonut(containerId, successRate, timeoutRate) {
            const container = document.getElementById(containerId);
            const successPct = successRate * 100;
            const timeoutPct = timeoutRate * 100;
            const errorPct = 100 - successPct - timeoutPct;

            const circumference = 2 * Math.PI * 15.9;
            const successDash = (successPct / 100) * circumference;
            const timeoutDash = (timeoutPct / 100) * circumference;
            const errorDash = (errorPct / 100) * circumference;

            let offset = 0;
            const successOffset = offset;
            offset += successDash;
            const timeoutOffset = offset;
            offset += timeoutDash;
            const errorOffset = offset;

            container.innerHTML = 
                '<svg viewBox="0 0 42 42" style="width: 100%; height: 100%;">' +
                '<circle class="donut-ring" cx="21" cy="21" r="15.9"/>' +
                '<circle class="donut-segment donut-success" cx="21" cy="21" r="15.9" ' +
                'stroke-dasharray="' + successDash + ' ' + (circumference - successDash) + '" ' +
                'stroke-dashoffset="' + (circumference / 4) + '"/>' +
                '<circle class="donut-segment donut-timeout" cx="21" cy="21" r="15.9" ' +
                'stroke-dasharray="' + timeoutDash + ' ' + (circumference - timeoutDash) + '" ' +
                'stroke-dashoffset="' + (circumference / 4 - successDash) + '"/>' +
                '<circle class="donut-segment donut-error" cx="21" cy="21" r="15.9" ' +
                'stroke-dasharray="' + errorDash + ' ' + (circumference - errorDash) + '" ' +
                'stroke-dashoffset="' + (circumference / 4 - successDash - timeoutDash) + '"/>' +
                '</svg>';
        }

        function renderRequests() {
            const loading = document.getElementById('loadingState');
            const table = document.getElementById('requestsTable');
            const empty = document.getElementById('emptyState');
            const pagination = document.getElementById('pagination');
            const tbody = document.getElementById('tableBody');

            loading.style.display = 'none';

            if (requestsData.length === 0) {
                table.style.display = 'none';
                empty.style.display = 'block';
                pagination.style.display = 'none';
                return;
            }

            table.style.display = 'table';
            empty.style.display = 'none';

            const start = (currentPage - 1) * pageSize;
            const end = start + pageSize;
            const pageData = requestsData.slice(start, end);
            const totalPages = Math.ceil(requestsData.length / pageSize);

            document.getElementById('prevPage').disabled = currentPage <= 1;
            document.getElementById('nextPage').disabled = currentPage >= totalPages;
            document.getElementById('pageInfo').textContent = 'Page ' + currentPage + ' of ' + totalPages;
            pagination.style.display = totalPages > 1 ? 'flex' : 'none';

            tbody.innerHTML = pageData.map(req => {
                const statusClass = getStatusClass(req.status, req.reason);
                const statusText = req.reason || req.status;
                return '<tr onclick="showRequestModal(\'' + req.id + '\')">' +
                    '<td>' + formatTime(req.ts) + '</td>' +
                    '<td>' + (req.model || '-') + '</td>' +
                    '<td>' + (req.endpoint || '-') + '</td>' +
                    '<td>' + formatDuration(req.duration_ms) + '</td>' +
                    '<td>' + formatBytes(req.bytes) + '</td>' +
                    '<td><span class="status-badge ' + statusClass + '">' + statusText + '</span></td>' +
                    '</tr>';
            }).join('');
        }

        async function showRequestModal(id) {
            const detail = await fetchRequestDetail(id);
            if (!detail) return;

            renderModalOverview(detail);
            renderModalTimings(detail);
            renderModalTokens(detail);
            renderModalBytes(detail);

            document.getElementById('requestModal').style.display = 'flex';
            document.querySelector('.modal-tab[data-tab="overview"]').click();
        }

        function renderModalOverview(d) {
            const container = document.getElementById('tab-overview');
            container.innerHTML = 
                '<div class="flow-diagram">' +
                '<div class="flow-box">' +
                '<div class="flow-box-title">Request</div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Messages</span><span class="flow-metric-value">' + d.request.messages_count + '</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">System</span><span class="flow-metric-value">' + formatNumber(d.request.system_chars) + ' chars</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">User</span><span class="flow-metric-value">' + formatNumber(d.request.user_chars) + ' chars</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Tools</span><span class="flow-metric-value">' + d.request.tools_count + '</span></div>' +
                '</div>' +
                '<div class="flow-arrow">→</div>' +
                '<div class="flow-box">' +
                '<div class="flow-box-title">AutoCTX</div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Estimated</span><span class="flow-metric-value">' + formatNumber(d.autoctx.ctx_est) + '</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Selected</span><span class="flow-metric-value">' + formatNumber(d.autoctx.ctx_selected) + '</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Bucket</span><span class="flow-metric-value">' + formatNumber(d.autoctx.ctx_bucket) + '</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Output Budget</span><span class="flow-metric-value">' + formatNumber(d.autoctx.output_budget) + '</span></div>' +
                '</div>' +
                '<div class="flow-arrow">→</div>' +
                '<div class="flow-box">' +
                '<div class="flow-box-title">Ollama</div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Prompt Tokens</span><span class="flow-metric-value">' + formatNumber(d.ollama.prompt_tokens) + '</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Completion</span><span class="flow-metric-value">' + formatNumber(d.ollama.completion_tokens) + '</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Eval Time</span><span class="flow-metric-value">' + formatDuration(d.ollama.upstream_eval_ms) + '</span></div>' +
                '</div>' +
                '<div class="flow-arrow">→</div>' +
                '<div class="flow-box">' +
                '<div class="flow-box-title">Response</div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Status</span><span class="flow-metric-value">' + d.status + '</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Duration</span><span class="flow-metric-value">' + formatDuration(d.response.duration_ms) + '</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">TTFB</span><span class="flow-metric-value">' + formatDuration(d.response.ttfb_ms) + '</span></div>' +
                '<div class="flow-metric"><span class="flow-metric-label">Retries</span><span class="flow-metric-value">' + d.response.retry_count + '</span></div>' +
                '</div>' +
                '</div>';
        }

        function renderModalTimings(d) {
            const container = document.getElementById('tab-timings');
            const total = d.response.duration_ms || 1;
            const ttfb = d.response.ttfb_ms || 0;
            const load = d.ollama.upstream_load_ms || 0;
            const promptEval = d.ollama.upstream_prompt_eval_ms || 0;
            const eval_ = d.ollama.upstream_eval_ms || 0;

            container.innerHTML = 
                '<div class="timing-bar-container">' +
                '<div class="timing-bar-label"><span>TTFB</span><span>' + formatDuration(ttfb) + '</span></div>' +
                '<div class="timing-bar"><div class="timing-bar-fill" style="width: ' + (ttfb/total*100) + '%; background: var(--accent-blue);"></div></div>' +
                '</div>' +
                '<div class="timing-bar-container">' +
                '<div class="timing-bar-label"><span>Model Load</span><span>' + formatDuration(load) + '</span></div>' +
                '<div class="timing-bar"><div class="timing-bar-fill" style="width: ' + (load/total*100) + '%; background: var(--accent-yellow);"></div></div>' +
                '</div>' +
                '<div class="timing-bar-container">' +
                '<div class="timing-bar-label"><span>Prompt Eval</span><span>' + formatDuration(promptEval) + '</span></div>' +
                '<div class="timing-bar"><div class="timing-bar-fill" style="width: ' + (promptEval/total*100) + '%; background: var(--accent-purple);"></div></div>' +
                '</div>' +
                '<div class="timing-bar-container">' +
                '<div class="timing-bar-label"><span>Generation</span><span>' + formatDuration(eval_) + '</span></div>' +
                '<div class="timing-bar"><div class="timing-bar-fill" style="width: ' + (eval_/total*100) + '%; background: var(--accent-green);"></div></div>' +
                '</div>' +
                '<div class="timing-bar-container">' +
                '<div class="timing-bar-label"><span><strong>Total Duration</strong></span><span><strong>' + formatDuration(total) + '</strong></span></div>' +
                '<div class="timing-bar"><div class="timing-bar-fill" style="width: 100%; background: var(--text-muted);"></div></div>' +
                '</div>';
        }

        function renderModalTokens(d) {
            const container = document.getElementById('tab-tokens');
            const utilization = d.autoctx.ctx_selected > 0 
                ? ((d.ollama.prompt_tokens + d.ollama.completion_tokens) / d.autoctx.ctx_selected * 100).toFixed(1)
                : 0;

            container.innerHTML = 
                '<div class="detail-grid">' +
                '<div class="detail-item"><div class="detail-label">Estimated Prompt</div><div class="detail-value">' + formatNumber(d.autoctx.ctx_est) + '</div></div>' +
                '<div class="detail-item"><div class="detail-label">Actual Prompt</div><div class="detail-value">' + formatNumber(d.ollama.prompt_tokens) + '</div></div>' +
                '<div class="detail-item"><div class="detail-label">Output Budget</div><div class="detail-value">' + formatNumber(d.autoctx.output_budget) + '</div></div>' +
                '<div class="detail-item"><div class="detail-label">Actual Output</div><div class="detail-value">' + formatNumber(d.ollama.completion_tokens) + '</div></div>' +
                '<div class="detail-item"><div class="detail-label">Context Selected</div><div class="detail-value">' + formatNumber(d.autoctx.ctx_selected) + '</div></div>' +
                '<div class="detail-item"><div class="detail-label">Context Bucket</div><div class="detail-value">' + formatNumber(d.autoctx.ctx_bucket) + '</div></div>' +
                '<div class="detail-item"><div class="detail-label">Context Utilization</div><div class="detail-value">' + utilization + '%</div></div>' +
                '<div class="detail-item"><div class="detail-label">Tokens/sec</div><div class="detail-value">' + 
                    (d.ollama.upstream_eval_ms > 0 ? (d.ollama.completion_tokens / d.ollama.upstream_eval_ms * 1000).toFixed(1) : '-') + '</div></div>' +
                '</div>';
        }

        function renderModalBytes(d) {
            const container = document.getElementById('tab-bytes');
            container.innerHTML = 
                '<div class="detail-grid">' +
                '<div class="detail-item"><div class="detail-label">Client Request</div><div class="detail-value">' + formatBytes(d.request.client_in_bytes) + '</div></div>' +
                '<div class="detail-item"><div class="detail-label">Client Response</div><div class="detail-value">' + formatBytes(d.response.client_out_bytes) + '</div></div>' +
                '<div class="detail-item"><div class="detail-label">Upstream Request</div><div class="detail-value">' + formatBytes(d.ollama.upstream_in_bytes) + '</div></div>' +
                '<div class="detail-item"><div class="detail-label">Upstream Response</div><div class="detail-value">' + formatBytes(d.ollama.upstream_out_bytes) + '</div></div>' +
                '</div>';
        }

        function closeModal() {
            document.getElementById('requestModal').style.display = 'none';
        }

        function updateHealth(healthy) {
            const dot = document.getElementById('healthDot');
            const text = document.getElementById('healthText');
            dot.className = 'health-dot' + (healthy ? '' : ' unhealthy');
            text.textContent = healthy ? 'Healthy' : 'Unhealthy';
        }

        // Formatting helpers
        function formatNumber(n) {
            if (n == null) return '-';
            return n.toLocaleString();
        }

        function formatDuration(ms) {
            if (ms == null || ms === 0) return '-';
            if (ms < 1000) return ms + 'ms';
            if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
            return (ms / 60000).toFixed(1) + 'm';
        }

        function formatBytes(bytes) {
            if (bytes == null || bytes === 0) return '-';
            if (bytes < 1024) return bytes + ' B';
            if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB';
            return (bytes / 1048576).toFixed(1) + ' MB';
        }

        function formatTime(ts) {
            if (!ts) return '-';
            const d = new Date(ts);
            return d.toLocaleTimeString();
        }

        function getStatusClass(status, reason) {
            if (status === 'success') return 'status-success';
            if (status === 'in_flight') return 'status-inflight';
            if (reason && reason.includes('timeout')) return 'status-timeout';
            return 'status-error';
        }
    </script>
</body>
</html>
`
