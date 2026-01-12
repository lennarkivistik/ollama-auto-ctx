package proxy

// dashboardHTML contains the complete HTML/CSS/JS for the monitoring dashboard
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
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: var(--bg-secondary);
            color: var(--text-primary);
            line-height: 1.6;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 24px;
        }

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

        h1 {
            color: var(--text-primary);
            font-size: 24px;
            font-weight: 600;
        }

        .health-badge {
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 6px 12px;
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 4px;
            font-size: 14px;
            font-weight: 500;
        }

        .health-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: var(--accent-green);
        }

        .health-dot.unhealthy {
            background: var(--accent-red);
        }

        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 16px;
            margin-bottom: 24px;
        }

        .metric-card {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            padding: 20px;
            border-radius: 8px;
        }

        .metric-label {
            color: var(--text-secondary);
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            margin-bottom: 12px;
            font-weight: 600;
        }

        .metric-value {
            color: var(--text-primary);
            font-size: 32px;
            font-weight: 600;
            margin-bottom: 4px;
            font-variant-numeric: tabular-nums;
        }

        .metric-subtext {
            color: var(--text-muted);
            font-size: 13px;
        }

        .requests-section {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 24px;
        }

        .section-header {
            margin-bottom: 20px;
        }

        .section-title {
            font-size: 18px;
            font-weight: 600;
            color: var(--text-primary);
            margin-bottom: 4px;
        }

        .section-subtitle {
            font-size: 14px;
            color: var(--text-muted);
        }

        .tabs {
            display: flex;
            gap: 0;
            margin-bottom: 20px;
            border-bottom: 1px solid var(--border-color);
        }

        .tab {
            padding: 10px 20px;
            background: none;
            border: none;
            color: var(--text-secondary);
            cursor: pointer;
            font-size: 14px;
            font-weight: 500;
            border-bottom: 2px solid transparent;
            margin-bottom: -1px;
            transition: color 0.2s;
        }

        .tab:hover {
            color: var(--text-primary);
        }

        .tab.active {
            color: var(--accent-blue);
            border-bottom-color: var(--accent-blue);
        }

        .table-container {
            overflow-x: auto;
        }

        table {
            width: 100%;
            border-collapse: collapse;
        }

        thead {
            background: var(--bg-secondary);
        }

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

        tr:hover {
            background: var(--bg-hover);
        }

        .status-badge {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            padding: 4px 8px;
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

        .status-success {
            background: rgba(16, 185, 129, 0.15);
            color: var(--accent-green);
        }

        .status-success::before {
            background: var(--accent-green);
        }

        .status-error {
            background: rgba(239, 68, 68, 0.15);
            color: var(--accent-red);
        }

        .status-error::before {
            background: var(--accent-red);
        }

        .status-timeout {
            background: rgba(245, 158, 11, 0.15);
            color: var(--accent-yellow);
        }

        .status-timeout::before {
            background: var(--accent-yellow);
        }

        .status-inflight {
            background: rgba(59, 130, 246, 0.15);
            color: var(--accent-blue);
        }

        .status-inflight::before {
            background: var(--accent-blue);
        }

        /* Tooltip for status badges */
        .status-badge {
            position: relative;
            cursor: help;
        }

        .status-badge:hover::after {
            content: attr(data-tooltip);
            position: absolute;
            bottom: 100%;
            left: 50%;
            transform: translateX(-50%);
            padding: 6px 10px;
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 4px;
            font-size: 12px;
            white-space: nowrap;
            z-index: 1000;
            margin-bottom: 5px;
            color: var(--text-primary);
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
            pointer-events: none;
        }

        .status-badge:hover::before {
            content: '';
            position: absolute;
            bottom: 100%;
            left: 50%;
            transform: translateX(-50%);
            border: 5px solid transparent;
            border-top-color: var(--border-color);
            margin-bottom: 0;
            z-index: 1001;
        }

        .empty-state {
            text-align: center;
            padding: 48px 24px;
            color: var(--text-muted);
        }

        .loading {
            text-align: center;
            padding: 24px;
            color: var(--text-secondary);
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

        .error {
            background: rgba(239, 68, 68, 0.1);
            color: var(--accent-red);
            padding: 12px 16px;
            border-radius: 4px;
            margin-bottom: 20px;
            border-left: 3px solid var(--accent-red);
        }

        .metrics-section {
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 24px;
            margin-top: 24px;
        }

        .metrics-grid-large {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 16px;
            margin-top: 20px;
        }

        .metric-item {
            display: flex;
            flex-direction: column;
            gap: 4px;
        }

        .metric-item-label {
            font-size: 12px;
            color: var(--text-secondary);
            text-transform: uppercase;
            letter-spacing: 0.5px;
            font-weight: 600;
        }

        .metric-item-value {
            font-size: 20px;
            font-weight: 600;
            color: var(--text-primary);
            font-variant-numeric: tabular-nums;
        }

        .metric-item-sub {
            font-size: 12px;
            color: var(--text-muted);
        }

        .metrics-breakdown {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 12px;
            margin-top: 20px;
            padding-top: 20px;
            border-top: 1px solid var(--border-color);
        }

        .breakdown-item {
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .breakdown-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
        }

        .breakdown-label {
            font-size: 13px;
            color: var(--text-secondary);
            flex: 1;
        }

        .breakdown-value {
            font-size: 14px;
            font-weight: 600;
            color: var(--text-primary);
            font-variant-numeric: tabular-nums;
        }

        @keyframes spin {
            to {
                transform: rotate(360deg);
            }
        }

        /* Responsive design */
        @media (max-width: 768px) {
            .container {
                padding: 16px;
            }

            header {
                flex-direction: column;
                gap: 12px;
                text-align: center;
            }

            .metrics-grid {
                grid-template-columns: 1fr;
            }

            .requests-section {
                padding: 16px;
            }
        }

        /* Scrollbar styling */
        ::-webkit-scrollbar {
            width: 8px;
            height: 8px;
        }

        ::-webkit-scrollbar-track {
            background: var(--bg-secondary);
        }

        ::-webkit-scrollbar-thumb {
            background: var(--border-color);
            border-radius: 4px;
        }

        ::-webkit-scrollbar-thumb:hover {
            background: var(--text-muted);
        }

        /* Pagination */
        .pagination {
            display: flex;
            justify-content: center;
            align-items: center;
            gap: 16px;
            margin-top: 16px;
            padding: 16px;
        }

        .pagination-btn {
            padding: 8px 16px;
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            color: var(--text-primary);
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
        }

        .pagination-btn:hover:not(:disabled) {
            background: var(--bg-hover);
        }

        .pagination-btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }

        .page-info {
            color: var(--text-secondary);
            font-size: 14px;
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
            max-width: 800px;
            width: 90%;
            max-height: 90vh;
            overflow-y: auto;
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.3);
        }

        .modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 20px 24px;
            border-bottom: 1px solid var(--border-color);
        }

        .modal-header h2 {
            margin: 0;
            font-size: 20px;
        }

        .modal-close {
            background: none;
            border: none;
            color: var(--text-secondary);
            font-size: 28px;
            cursor: pointer;
            padding: 0;
            width: 32px;
            height: 32px;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 4px;
        }

        .modal-close:hover {
            background: var(--bg-hover);
            color: var(--text-primary);
        }

        .modal-body {
            padding: 24px;
        }

        .modal-detail-row {
            display: grid;
            grid-template-columns: 200px 1fr;
            gap: 16px;
            padding: 12px 0;
            border-bottom: 1px solid var(--border-color);
        }

        .modal-detail-row:last-child {
            border-bottom: none;
        }

        .modal-detail-label {
            font-weight: 600;
            color: var(--text-secondary);
            font-size: 13px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .modal-detail-value {
            color: var(--text-primary);
            word-break: break-word;
        }

        /* Clickable table rows */
        tbody tr {
            cursor: pointer;
        }

        tbody tr:hover {
            background: var(--bg-hover);
        }

        /* Config grid */
        /* Config button */
        .config-button {
            display: flex;
            align-items: center;
            gap: 6px;
            padding: 6px 12px;
            background: var(--bg-card);
            border: 1px solid var(--border-color);
            color: var(--text-secondary);
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            transition: all 0.2s;
        }

        .config-button:hover {
            background: var(--bg-hover);
            color: var(--text-primary);
            border-color: var(--accent-blue);
        }

        /* Config modal tabs */
        .config-tabs {
            display: flex;
            gap: 8px;
            margin-bottom: 24px;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 0;
        }

        .config-tab {
            padding: 10px 16px;
            background: none;
            border: none;
            border-bottom: 2px solid transparent;
            color: var(--text-secondary);
            cursor: pointer;
            font-size: 13px;
            font-weight: 500;
            transition: all 0.2s;
            margin-bottom: -1px;
        }

        .config-tab:hover {
            color: var(--text-primary);
        }

        .config-tab.active {
            color: var(--accent-blue);
            border-bottom-color: var(--accent-blue);
        }

        /* Config content - elegant list style like Prometheus */
        .config-content {
            display: none;
        }

        .config-content.active {
            display: block;
        }

        .config-list {
            display: flex;
            flex-direction: column;
            gap: 0;
        }

        .config-list-item {
            display: grid;
            grid-template-columns: 250px 1fr;
            gap: 24px;
            padding: 12px 0;
            border-bottom: 1px solid var(--border-color);
        }

        .config-list-item:last-child {
            border-bottom: none;
        }

        .config-list-label {
            font-size: 13px;
            color: var(--text-secondary);
            font-weight: 500;
        }

        .config-list-value {
            font-size: 13px;
            color: var(--text-primary);
            word-break: break-word;
            font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
        }

        /* View toggle */
        .view-toggle {
            display: flex;
            gap: 8px;
            justify-content: flex-end;
        }

        .view-toggle-btn {
            padding: 6px 12px;
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            color: var(--text-secondary);
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
            transition: all 0.2s;
        }

        .view-toggle-btn:hover {
            background: var(--bg-hover);
        }

        .view-toggle-btn.active {
            background: var(--accent-blue);
            color: var(--text-primary);
            border-color: var(--accent-blue);
        }

        /* 24-hour bar chart */
        .chart-container {
            margin-top: 20px;
        }

        .chart-bars {
            display: flex;
            align-items: flex-end;
            gap: 4px;
            height: 200px;
            padding: 16px 0 40px 0;
        }

        .chart-bar {
            flex: 1;
            background: var(--accent-blue);
            border-radius: 2px 2px 0 0;
            min-height: 2px;
            position: relative;
            transition: opacity 0.2s;
        }

        .chart-bar:hover {
            opacity: 0.8;
        }

        .chart-bar-wrapper {
            display: flex;
            flex-direction: column;
            align-items: center;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>ollama-auto-ctx</h1>
            <div style="display: flex; align-items: center; gap: 16px;">
                <button class="config-button" onclick="showConfigModal()" title="View Configuration">
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2">
                        <circle cx="8" cy="8" r="2"/>
                        <path d="M8 2v2M8 12v2M2 8h2M12 8h2M3.5 3.5l1.4 1.4M11.1 11.1l1.4 1.4M3.5 12.5l1.4-1.4M11.1 4.9l1.4-1.4"/>
                    </svg>
                    Config
                </button>
                <div class="health-badge">
                    <div class="health-dot" id="healthDot"></div>
                    <span id="healthText">Checking...</span>
                </div>
            </div>
        </header>

        <div class="metrics-grid">
            <div class="metric-card">
                <div class="metric-label">In-Flight Requests</div>
                <div class="metric-value" id="inFlightCount">0</div>
                <div class="metric-subtext">Currently processing</div>
            </div>
            <div class="metric-card">
                <div class="metric-label">Recent Requests</div>
                <div class="metric-value" id="recentCount">0</div>
                <div class="metric-subtext">In the last session</div>
            </div>
            <div class="metric-card">
                <div class="metric-label">Upstream Status</div>
                <div class="metric-value" id="upstreamStatus" style="font-size: 18px; margin-top: 8px;">-</div>
                <div class="metric-subtext">Ollama connection</div>
            </div>
        </div>

        <!-- 24-Hour Request Chart -->
        <div class="metrics-section">
            <div class="section-header">
                <div class="section-title">24-Hour Request Activity</div>
                <div class="section-subtitle">Requests per hour over the last 24 hours</div>
            </div>

            <div class="chart-container">
                <div class="chart-bars" id="hourlyChart">
                    <!-- Bars will be generated here -->
                </div>
            </div>
        </div>

        <div class="requests-section">
            <div class="section-header">
                <div class="section-title">Request Monitor</div>
                <div class="section-subtitle">Real-time request tracking and analytics</div>
            </div>

            <div class="tabs">
                <button class="tab" data-tab="inflight">In-Flight</button>
                <button class="tab active" data-tab="recent">Recent</button>
            </div>

            <div class="view-toggle" style="margin-top: 16px; margin-bottom: 16px;">
                <button class="view-toggle-btn active" data-view="request">Request View</button>
                <button class="view-toggle-btn" data-view="token">Token View</button>
            </div>

            <div id="errorMessage" class="error" style="display: none;"></div>

            <div class="table-container">
                <div id="loadingState" class="loading">Loading requests...</div>
                <table id="requestsTable" style="display: none;">
                    <thead id="tableHeader">
                        <tr>
                            <th>Date</th>
                            <th>Model</th>
                            <th>Selected Tokens</th>
                            <th>Output Tokens</th>
                            <th>Duration</th>
                            <th>Status</th>
                        </tr>
                    </thead>
                    <tbody id="tableBody"></tbody>
                </table>
                <div id="emptyState" class="empty-state" style="display: none;">
                    No requests to display
                </div>
                <div id="pagination" class="pagination" style="display: none;">
                    <button id="prevPage" class="pagination-btn" disabled>Previous</button>
                    <span id="pageInfo" class="page-info">Page 1 of 1</span>
                    <button id="nextPage" class="pagination-btn" disabled>Next</button>
                </div>
            </div>
        </div>

        <!-- Request Details Modal -->
        <div id="requestModal" class="modal" style="display: none;">
            <div class="modal-content">
                <div class="modal-header">
                    <h2>Request Details</h2>
                    <button class="modal-close" onclick="closeModal()">&times;</button>
                </div>
                <div class="modal-body" id="modalBody">
                    <!-- Details will be populated here -->
                </div>
            </div>
        </div>

        <div class="metrics-section">
            <div class="section-header">
                <div class="section-title">Prometheus Metrics</div>
                <div class="section-subtitle">Aggregated statistics and performance indicators</div>
            </div>

            <div class="metrics-grid-large" id="prometheusMetrics">
                <div class="metric-item">
                    <div class="metric-item-label">Total Requests</div>
                    <div class="metric-item-value" id="totalRequests">-</div>
                    <div class="metric-item-sub">All time</div>
                </div>
                <div class="metric-item">
                    <div class="metric-item-label">Success Rate</div>
                    <div class="metric-item-value" id="successRate">-</div>
                    <div class="metric-item-sub" id="successRateSub">-</div>
                </div>
                <div class="metric-item">
                    <div class="metric-item-label">Avg Duration</div>
                    <div class="metric-item-value" id="avgDuration">-</div>
                    <div class="metric-item-sub">Request time</div>
                </div>
                <div class="metric-item">
                    <div class="metric-item-label">Total Bytes</div>
                    <div class="metric-item-value" id="totalBytes">-</div>
                    <div class="metric-item-sub">Forwarded</div>
                </div>
                <div class="metric-item">
                    <div class="metric-item-label">Total Tokens</div>
                    <div class="metric-item-value" id="totalTokens">-</div>
                    <div class="metric-item-sub">Estimated</div>
                </div>
                <div class="metric-item">
                    <div class="metric-item-label">Timeouts</div>
                    <div class="metric-item-value" id="totalTimeouts">-</div>
                    <div class="metric-item-sub">All types</div>
                </div>
            </div>

            <div class="metrics-breakdown" id="metricsBreakdown" style="display: none;">
                <div class="breakdown-item">
                    <div class="breakdown-dot" style="background: var(--accent-green);"></div>
                    <div class="breakdown-label">Success</div>
                    <div class="breakdown-value" id="breakdownSuccess">0</div>
                </div>
                <div class="breakdown-item">
                    <div class="breakdown-dot" style="background: var(--accent-red);"></div>
                    <div class="breakdown-label">Errors</div>
                    <div class="breakdown-value" id="breakdownErrors">0</div>
                </div>
                <div class="breakdown-item">
                    <div class="breakdown-dot" style="background: var(--accent-yellow);"></div>
                    <div class="breakdown-label">Timeouts</div>
                    <div class="breakdown-value" id="breakdownTimeouts">0</div>
                </div>
                <div class="breakdown-item">
                    <div class="breakdown-dot" style="background: var(--accent-orange);"></div>
                    <div class="breakdown-label">Loops Detected</div>
                    <div class="breakdown-value" id="breakdownLoops">0</div>
                </div>
            </div>
        </div>

        <!-- Recent Errored Requests Section -->
        <div class="metrics-section">
            <div class="section-header">
                <div class="section-title">Recent Errors</div>
                <div class="section-subtitle">Last 10 failed requests with detailed error information</div>
            </div>

            <div class="table-container">
                <table id="erroredRequestsTable">
                    <thead>
                        <tr>
                            <th>Date</th>
                            <th>Model</th>
                            <th>Error Type</th>
                            <th>Duration</th>
                            <th>Details</th>
                        </tr>
                    </thead>
                    <tbody id="erroredRequestsBody">
                        <tr>
                            <td colspan="5" style="text-align: center; padding: 24px; color: var(--text-muted);">No errors yet</td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>

        <!-- Configuration Modal -->
        <div id="configModal" class="modal" style="display: none;">
            <div class="modal-content" style="max-width: 1000px;">
                <div class="modal-header">
                    <h2>Configuration</h2>
                    <button class="modal-close" onclick="closeConfigModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="config-tabs" id="configTabs">
                        <!-- Tabs will be generated here -->
                    </div>
                    <div class="config-content" id="configContent">
                        <!-- Content will be populated here -->
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        let currentTab = 'recent';
        let requestsData = { inFlight: {}, recent: [] };
        let eventSource = null;

        // Throttling/debouncing for updates
        let fetchRequestsTimeout = null;
        let renderTableTimeout = null;
        let lastFetchTime = 0;
        let lastRenderTime = 0;
        const FETCH_THROTTLE_MS = 1000; // Max once per second
        const RENDER_THROTTLE_MS = 500; // Max once per 500ms

        // Pagination state
        let currentPage = 1;
        const ITEMS_PER_PAGE = 20;
        
        // Request data map for modal (keyed by request ID)
        let requestDataMap = new Map();

        // View state (request or token)
        let currentView = 'request';

        // 24-hour request tracking (stores {hour: timestamp, count: number})
        let hourlyRequests = [];

        // Session statistics (in-memory only)
        let sessionStats = {
            totalRequests: 0,
            completedRequests: 0,
            successCount: 0,
            failureCount: 0,
            timeoutCount: 0,
            completionTimes: []
        };

        // Rolling list of last 10 completed requests
        let recentCompletedRequests = [];
        const MAX_RECENT_REQUESTS = 10;

        // Track request start (no longer tracking session count)
        function trackRequestStart() {
            // Removed session request count tracking - only use backend truth
        }

        // Track request completion
        function trackRequestCompletion(status, duration, requestId, model, contextData) {
            sessionStats.completedRequests++;
            
            if (duration) {
                sessionStats.completionTimes.push(duration);
                // Keep only last 1000 completion times to avoid memory issues
                if (sessionStats.completionTimes.length > 1000) {
                    sessionStats.completionTimes.shift();
                }
            }
            
            if (status === 'success') {
                sessionStats.successCount++;
            } else if (status === 'timeout_ttfb' || status === 'timeout_stall' || status === 'timeout_hard') {
                sessionStats.timeoutCount++;
                sessionStats.failureCount++;
            } else if (status === 'upstream_error' || status === 'loop_detected' || status === 'output_limit_exceeded' || status === 'canceled') {
                sessionStats.failureCount++;
            }
            
            // Add to recent completed requests (rolling window of 10)
            if (requestId) {
                addToRecentRequests({
                    id: requestId,
                    model: model || 'unknown',
                    estimatedPromptTokens: contextData?.estimatedPromptTokens || null,
                    chosenCtx: contextData?.chosenCtx || null,
                    outputBudgetTokens: contextData?.outputBudgetTokens || null,
                    promptEvalCount: contextData?.promptEvalCount || null,
                    evalCount: contextData?.evalCount || null,
                    duration: duration,
                    status: status
                });
            }
            
        }

        // Add completed request to rolling list
        function addToRecentRequests(request) {
            // Remove if already exists (update)
            recentCompletedRequests = recentCompletedRequests.filter(r => r.id !== request.id);
            
            // Add to front
            recentCompletedRequests.unshift(request);
            
            // Keep only last 10
            if (recentCompletedRequests.length > MAX_RECENT_REQUESTS) {
                recentCompletedRequests = recentCompletedRequests.slice(0, MAX_RECENT_REQUESTS);
            }
        }

        // Format duration
        function formatDuration(ms) {
            if (!ms) return '-';
            if (ms < 1000) return ms + 'ms';
            if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
            return (ms / 60000).toFixed(1) + 'm';
        }

        // Format bytes
        function formatBytes(bytes) {
            if (!bytes) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
        }

        // Format number
        function formatNumber(num) {
            if (!num) return '0';
            return num.toLocaleString();
        }

        // Get status badge class
        function getStatusClass(status) {
            if (!status) return 'status-inflight';
            if (status === 'success') return 'status-success';
            if (status.startsWith('timeout')) return 'status-timeout';
            if (status === 'upstream_error' || status === 'loop_detected' || status === 'output_limit_exceeded') return 'status-error';
            return 'status-inflight';
        }

        // Get status text
        function getStatusText(status) {
            if (!status) return 'In-Flight';
            return status.replace(/_/g, ' ');
        }

        // Get status tooltip description
        function getStatusTooltip(status) {
            const tooltips = {
                'success': 'Request completed successfully',
                'timeout_ttfb': 'Timeout: No first byte received within the configured TTFB timeout period',
                'timeout_stall': 'Timeout: No activity detected after first byte within the configured stall timeout period',
                'timeout_hard': 'Timeout: Total request duration exceeded the configured hard timeout limit',
                'upstream_error': 'Error: The upstream Ollama server returned an error',
                'loop_detected': 'Error: Degenerate repeating output pattern detected',
                'output_limit_exceeded': 'Error: Output token limit exceeded',
                'canceled': 'Request was canceled',
                '': 'Request is currently in-flight and being processed'
            };
            return tooltips[status] || 'Unknown status';
        }

        // Calculate duration
        function calculateDuration(startTime, endTime) {
            if (!startTime) return null;
            const start = new Date(startTime);
            const end = endTime ? new Date(endTime) : new Date();
            return end - start;
        }

        // Render table
        function renderTable() {
            const tbody = document.getElementById('tableBody');
            const loadingState = document.getElementById('loadingState');
            const table = document.getElementById('requestsTable');
            const emptyState = document.getElementById('emptyState');
            const pagination = document.getElementById('pagination');

            loadingState.style.display = 'none';

            let rows = [];
            if (currentTab === 'inflight') {
                rows = Object.values(requestsData.inFlight);
            } else {
                // Sort recent requests by newest first (by start_time descending)
                rows = [...requestsData.recent].sort((a, b) => {
                    const timeA = a.start_time ? new Date(a.start_time).getTime() : 0;
                    const timeB = b.start_time ? new Date(b.start_time).getTime() : 0;
                    return timeB - timeA; // Descending order (newest first)
                });
            }

            if (rows.length === 0) {
                table.style.display = 'none';
                emptyState.style.display = 'block';
                pagination.style.display = 'none';
                return;
            }

            table.style.display = 'table';
            emptyState.style.display = 'none';

            // Pagination
            const totalPages = Math.ceil(rows.length / ITEMS_PER_PAGE);
            const startIdx = (currentPage - 1) * ITEMS_PER_PAGE;
            const endIdx = startIdx + ITEMS_PER_PAGE;
            const paginatedRows = rows.slice(startIdx, endIdx);

            // Update pagination controls
            document.getElementById('prevPage').disabled = currentPage === 1;
            document.getElementById('nextPage').disabled = currentPage >= totalPages;
            document.getElementById('pageInfo').textContent = 'Page ' + currentPage + ' of ' + totalPages;
            pagination.style.display = totalPages > 1 ? 'flex' : 'none';

            // Update table headers based on view
            const tableHeader = document.getElementById('tableHeader');
            if (currentView === 'token') {
                tableHeader.innerHTML = '<tr><th>Date</th><th>Model</th><th>Est. Prompt</th><th>Selected Ctx</th><th>Output Budget</th><th>Input Tokens</th><th>Output Tokens</th><th>Duration</th><th>Status</th></tr>';
            } else {
                tableHeader.innerHTML = '<tr><th>Date</th><th>Model</th><th>Endpoint</th><th>Duration</th><th>Bytes</th><th>Status</th></tr>';
            }

            tbody.innerHTML = paginatedRows.map(req => {
                // Store request data for modal
                requestDataMap.set(req.id, req);
                
                // For completed requests, use last_activity_time as end time
                // For in-flight requests, use current time
                const endTime = (req.status && currentTab === 'recent') ? req.last_activity_time : null;
                const duration = calculateDuration(req.start_time, endTime);
                const status = req.status || '';
                const statusClass = getStatusClass(status);
                const statusText = getStatusText(status);
                const statusTooltip = getStatusTooltip(status);
                
                // Format date
                const date = req.start_time ? new Date(req.start_time).toLocaleString() : '-';
                
                if (currentView === 'token') {
                    // Token-centric view
                    return '<tr onclick="showRequestModal(\'' + req.id + '\')" style="cursor: pointer;">' +
                        '<td>' + date + '</td>' +
                        '<td>' + (req.model || '-') + '</td>' +
                        '<td>' + (req.estimated_prompt_tokens ? formatNumber(req.estimated_prompt_tokens) : '-') + '</td>' +
                        '<td>' + (req.chosen_ctx ? formatNumber(req.chosen_ctx) : '-') + '</td>' +
                        '<td>' + (req.output_budget_tokens ? formatNumber(req.output_budget_tokens) : '-') + '</td>' +
                        '<td>' + (req.prompt_eval_count ? formatNumber(req.prompt_eval_count) : '-') + '</td>' +
                        '<td>' + (req.eval_count ? formatNumber(req.eval_count) : '-') + '</td>' +
                        '<td>' + formatDuration(duration) + '</td>' +
                        '<td><span class="status-badge ' + statusClass + '" data-tooltip="' + statusTooltip + '">' + statusText + '</span></td>' +
                        '</tr>';
                } else {
                    // Request view
                    return '<tr onclick="showRequestModal(\'' + req.id + '\')" style="cursor: pointer;">' +
                        '<td>' + date + '</td>' +
                        '<td>' + (req.model || '-') + '</td>' +
                        '<td>' + (req.endpoint || '-') + '</td>' +
                        '<td>' + formatDuration(duration) + '</td>' +
                        '<td>' + formatBytes(req.bytes_forwarded || 0) + '</td>' +
                        '<td><span class="status-badge ' + statusClass + '" data-tooltip="' + statusTooltip + '">' + statusText + '</span></td>' +
                        '</tr>';
                }
            }).join('');
        }

        // Update metrics
        function updateMetrics() {
            document.getElementById('inFlightCount').textContent = Object.keys(requestsData.inFlight).length;
            document.getElementById('recentCount').textContent = requestsData.recent.length;
        }

        // Track completed requests from recent array (only once per request)
        const trackedRequestIds = new Set();
        const trackedStartIds = new Set();
        
        function trackRecentRequests(recentRequests, inFlightRequests) {
            // Track starts from in-flight requests
            for (const reqId in inFlightRequests) {
                if (trackedStartIds.has(reqId)) continue;
                trackedStartIds.add(reqId);
                trackRequestStart();
            }
            
                // Track completions from recent requests
                for (const req of recentRequests) {
                    if (trackedRequestIds.has(req.id)) continue;
                    trackedRequestIds.add(req.id);
                    
                    // Track by hour for chart
                    if (req.start_time) {
                        trackRequestByHour(req.start_time);
                    }
                    
                    // Only track if request has a status (completed)
                    if (req.status) {
                        let duration = null;
                        if (req.start_time && req.last_activity_time) {
                            // For completed requests, use last_activity_time as the completion time
                            const startTime = new Date(req.start_time);
                            const endTime = new Date(req.last_activity_time);
                            duration = endTime - startTime;
                        }
                        
                        // Extract context and token data
                        const contextData = {
                            estimatedPromptTokens: req.estimated_prompt_tokens || null,
                            chosenCtx: req.chosen_ctx || null,
                            outputBudgetTokens: req.output_budget_tokens || null,
                            promptEvalCount: req.prompt_eval_count || null,
                            evalCount: req.eval_count || null
                        };
                        
                        trackRequestCompletion(req.status, duration, req.id, req.model, contextData);
                    }
                }
        }

        // Throttled fetch requests (max once per FETCH_THROTTLE_MS)
        function throttledFetchRequests() {
            const now = Date.now();
            if (now - lastFetchTime < FETCH_THROTTLE_MS) {
                // Clear existing timeout and set a new one
                if (fetchRequestsTimeout) {
                    clearTimeout(fetchRequestsTimeout);
                }
                fetchRequestsTimeout = setTimeout(() => {
                    throttledFetchRequests();
                }, FETCH_THROTTLE_MS - (now - lastFetchTime));
                return;
            }
            lastFetchTime = now;
            fetchRequests();
        }

        // Throttled render table (max once per RENDER_THROTTLE_MS)
        function throttledRenderTable() {
            const now = Date.now();
            if (now - lastRenderTime < RENDER_THROTTLE_MS) {
                // Clear existing timeout and set a new one
                if (renderTableTimeout) {
                    clearTimeout(renderTableTimeout);
                }
                renderTableTimeout = setTimeout(() => {
                    throttledRenderTable();
                }, RENDER_THROTTLE_MS - (now - lastRenderTime));
                return;
            }
            lastRenderTime = now;
            renderTable();
        }

        // Fetch requests data
        async function fetchRequests() {
            try {
                const response = await fetch('/debug/requests');
                if (!response.ok) throw new Error('Failed to fetch requests');
                const data = await response.json();
                requestsData.inFlight = data.in_flight || {};
                requestsData.recent = data.recent || [];
                
                // Track stats from requests
                trackRecentRequests(requestsData.recent, requestsData.inFlight);
                
                updateMetrics();
                renderTable();
                renderErroredRequests(); // Update errored requests
            } catch (error) {
                showError('Failed to load requests: ' + error.message);
            }
        }

        // Fetch health status
        async function fetchHealth() {
            try {
                const response = await fetch('/healthz/upstream');
                if (!response.ok) {
                    updateHealth(false, 'Unhealthy');
                    return;
                }
                const data = await response.json();
                updateHealth(data.healthy, data.healthy ? 'Healthy' : 'Unhealthy');
            } catch (error) {
                updateHealth(false, 'Unknown');
            }
        }

        // Update health display
        function updateHealth(healthy, text) {
            const dot = document.getElementById('healthDot');
            const textEl = document.getElementById('healthText');
            dot.className = 'health-dot' + (healthy ? '' : ' unhealthy');
            textEl.textContent = text;
            document.getElementById('upstreamStatus').textContent = text;
        }

        // Show error
        function showError(message) {
            const errorEl = document.getElementById('errorMessage');
            errorEl.textContent = message;
            errorEl.style.display = 'block';
            setTimeout(() => {
                errorEl.style.display = 'none';
            }, 5000);
        }

        // Parse Prometheus metrics text format
        function parsePrometheusMetrics(text) {
            const metrics = {};
            const lines = text.split('\n');
            
            for (const line of lines) {
                if (line.trim() === '' || line.startsWith('#')) continue;
                
                const match = line.match(/^([a-z_]+)(?:\{([^}]+)\})?\s+(.+)$/);
                if (!match) continue;
                
                const [, name, labels, value] = match;
                const numValue = parseFloat(value);
                if (isNaN(numValue)) continue;
                
                if (!metrics[name]) {
                    metrics[name] = [];
                }
                
                const labelObj = {};
                if (labels) {
                    labels.split(',').forEach(l => {
                        const [key, val] = l.split('=');
                        if (key && val) {
                            labelObj[key.trim()] = val.trim().replace(/^"|"$/g, '');
                        }
                    });
                }
                
                metrics[name].push({ labels: labelObj, value: numValue });
            }
            
            return metrics;
        }

        // Sum values from a metric array
        function sumMetric(metricArray) {
            if (!metricArray) return 0;
            return metricArray.reduce((sum, item) => sum + item.value, 0);
        }

        // Get metric value by label
        function getMetricByLabel(metricArray, labelKey, labelValue) {
            if (!metricArray) return null;
            const item = metricArray.find(m => m.labels[labelKey] === labelValue);
            return item ? item.value : 0;
        }

        // Calculate average from histogram buckets
        function calculateHistogramAverage(metricArray) {
            if (!metricArray || metricArray.length === 0) return 0;
            // For simplicity, use the sum of all bucket values
            // In a real implementation, you'd calculate proper percentiles
            const sum = sumMetric(metricArray);
            const count = metricArray.length;
            return count > 0 ? sum / count : 0;
        }

        // Fetch and display Prometheus metrics
        async function fetchPrometheusMetrics() {
            try {
                const response = await fetch('/metrics');
                if (!response.ok) {
                    document.getElementById('prometheusMetrics').style.display = 'none';
                    return;
                }
                
                const text = await response.text();
                const metrics = parsePrometheusMetrics(text);
                
                // Total requests
                const requestsTotal = sumMetric(metrics['ollama_proxy_requests_total']);
                document.getElementById('totalRequests').textContent = formatNumber(requestsTotal);
                
                // Success vs errors
                const successCount = sumMetric(
                    metrics['ollama_proxy_requests_total']?.filter(m => m.labels.status === 'success') || []
                );
                const errorCount = sumMetric(
                    metrics['ollama_proxy_requests_total']?.filter(m => 
                        m.labels.status === 'upstream_error' || 
                        m.labels.status === 'timeout_ttfb' ||
                        m.labels.status === 'timeout_stall' ||
                        m.labels.status === 'timeout_hard'
                    ) || []
                );
                const timeoutCount = sumMetric(
                    metrics['ollama_proxy_requests_total']?.filter(m => 
                        m.labels.status?.startsWith('timeout')
                    ) || []
                );
                
                const successRate = requestsTotal > 0 ? ((successCount / requestsTotal) * 100).toFixed(1) : 0;
                document.getElementById('successRate').textContent = successRate + '%';
                document.getElementById('successRateSub').textContent = successCount + ' / ' + requestsTotal;
                
                // Average duration (using histogram _sum and _count)
                const durationSum = metrics['ollama_proxy_request_duration_seconds_sum']?.[0]?.value || 0;
                const durationCount = metrics['ollama_proxy_request_duration_seconds_count']?.[0]?.value || 0;
                const avgDuration = durationCount > 0 ? (durationSum / durationCount) : 0;
                document.getElementById('avgDuration').textContent = avgDuration > 0 ? 
                    (avgDuration < 1 ? (avgDuration * 1000).toFixed(0) + 'ms' : avgDuration.toFixed(2) + 's') : '-';
                
                // Total bytes
                const totalBytes = sumMetric(metrics['ollama_proxy_bytes_out_total']);
                document.getElementById('totalBytes').textContent = formatBytes(totalBytes);
                
                // Total tokens
                const totalTokens = sumMetric(metrics['ollama_proxy_tokens_estimated_total']);
                document.getElementById('totalTokens').textContent = formatNumber(totalTokens);
                
                // Timeouts
                const timeouts = sumMetric(metrics['ollama_proxy_timeouts_total']);
                document.getElementById('totalTimeouts').textContent = formatNumber(timeouts);
                
                // Breakdown
                document.getElementById('breakdownSuccess').textContent = formatNumber(successCount);
                document.getElementById('breakdownErrors').textContent = formatNumber(errorCount);
                document.getElementById('breakdownTimeouts').textContent = formatNumber(timeoutCount);
                
                const loopsDetected = metrics['ollama_proxy_loops_detected_total']?.[0]?.value || 0;
                document.getElementById('breakdownLoops').textContent = formatNumber(loopsDetected);
                
                if (requestsTotal > 0) {
                    document.getElementById('metricsBreakdown').style.display = 'grid';
                }
            } catch (error) {
                console.error('Failed to fetch metrics:', error);
            }
        }

        // Connect to SSE
        function connectSSE() {
            if (eventSource) {
                eventSource.close();
            }

            eventSource = new EventSource('/events');
            
            eventSource.onopen = () => {
                console.log('SSE connected');
            };

            eventSource.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    handleEvent(data);
                } catch (error) {
                    console.error('Failed to parse SSE event:', error);
                }
            };

            eventSource.onerror = (error) => {
                console.error('SSE error:', error);
                eventSource.close();
                // Reconnect after 3 seconds
                setTimeout(connectSSE, 3000);
            };
        }

        // Handle SSE event
        function handleEvent(event) {
            // Track request start
            if (event.type === 'request_start') {
                trackRequestStart();
                if (event.timestamp) {
                    trackRequestByHour(event.timestamp);
                }
            }
            
            // Track request completion
            if (event.type === 'done' || event.type === 'canceled' || 
                event.type === 'timeout_ttfb' || event.type === 'timeout_stall' || 
                event.type === 'timeout_hard' || event.type === 'upstream_error' || 
                event.type === 'loop_detected' || event.type === 'output_limit_exceeded') {
                
                // Calculate duration and get request details
                let duration = null;
                let model = null;
                let contextData = null;
                
                if (event.request_id) {
                    // Try to get details from in-flight request
                    const req = requestsData.inFlight[event.request_id];
                    if (req) {
                        model = req.model || event.model || null;
                        
                        // Extract context and token data from request
                        contextData = {
                            estimatedPromptTokens: req.estimated_prompt_tokens || null,
                            chosenCtx: req.chosen_ctx || null,
                            outputBudgetTokens: req.output_budget_tokens || null,
                            promptEvalCount: req.prompt_eval_count || null,
                            evalCount: req.eval_count || null
                        };
                        
                        if (req.start_time && event.timestamp) {
                            const startTime = new Date(req.start_time);
                            const endTime = new Date(event.timestamp);
                            duration = endTime - startTime;
                        }
                    } else {
                        // Fallback to event data
                        model = event.model || null;
                    }
                }
                
                const status = event.type === 'done' ? 'success' : event.type;
                trackRequestCompletion(status, duration, event.request_id, model, contextData);
            }
            
            // Refresh requests data on significant events (throttled)
            if (event.type === 'request_start' || event.type === 'done' || 
                event.type === 'canceled' || event.type.startsWith('timeout') ||
                event.type === 'upstream_error' || event.type === 'loop_detected') {
                throttledFetchRequests();
            } else if (event.type === 'progress' || event.type === 'first_byte') {
                // Update in-flight request if it exists (throttled updates)
                if (requestsData.inFlight[event.request_id]) {
                    const req = requestsData.inFlight[event.request_id];
                    if (event.bytes_out !== undefined) req.bytes_forwarded = event.bytes_out;
                    if (event.estimated_output_tokens !== undefined) req.estimated_output_tokens = event.estimated_output_tokens;
                    if (event.type === 'first_byte' && event.ttfb_ms !== undefined) {
                        req.first_byte_time = new Date(Date.now() - event.ttfb_ms).toISOString();
                    }
                    // Only update metrics immediately, throttle table rendering
                    updateMetrics();
                    throttledRenderTable();
                }
            }
        }

        // Modal functions
        function showRequestModal(requestId) {
            const req = requestDataMap.get(requestId);
            if (!req) return;
            
            const modal = document.getElementById('requestModal');
            const modalBody = document.getElementById('modalBody');
            
            const endTime = (req.status && currentTab === 'recent') ? req.last_activity_time : null;
            const duration = calculateDuration(req.start_time, endTime);
            const date = req.start_time ? new Date(req.start_time).toLocaleString() : '-';
            const endDate = req.last_activity_time ? new Date(req.last_activity_time).toLocaleString() : '-';
            
            modalBody.innerHTML = 
                '<div class="modal-detail-row"><div class="modal-detail-label">Request ID</div><div class="modal-detail-value">' + (req.id || '-') + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Date</div><div class="modal-detail-value">' + date + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">End Time</div><div class="modal-detail-value">' + endDate + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Model</div><div class="modal-detail-value">' + (req.model || '-') + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Endpoint</div><div class="modal-detail-value">' + (req.endpoint || '-') + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Status</div><div class="modal-detail-value"><span class="status-badge ' + getStatusClass(req.status || '') + '" data-tooltip="' + getStatusTooltip(req.status || '') + '">' + getStatusText(req.status || '') + '</span></div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Duration</div><div class="modal-detail-value">' + formatDuration(duration) + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Bytes Forwarded</div><div class="modal-detail-value">' + formatBytes(req.bytes_forwarded || 0) + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Estimated Prompt Tokens</div><div class="modal-detail-value">' + (req.estimated_prompt_tokens ? formatNumber(req.estimated_prompt_tokens) : '-') + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Chosen Context</div><div class="modal-detail-value">' + (req.chosen_ctx ? formatNumber(req.chosen_ctx) : '-') + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Output Budget Tokens</div><div class="modal-detail-value">' + (req.output_budget_tokens ? formatNumber(req.output_budget_tokens) : '-') + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Input Tokens (Actual)</div><div class="modal-detail-value">' + (req.prompt_eval_count ? formatNumber(req.prompt_eval_count) : '-') + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Output Tokens (Actual)</div><div class="modal-detail-value">' + (req.eval_count ? formatNumber(req.eval_count) : '-') + '</div></div>' +
                '<div class="modal-detail-row"><div class="modal-detail-label">Estimated Output Tokens</div><div class="modal-detail-value">' + (req.estimated_output_tokens ? formatNumber(req.estimated_output_tokens) : '-') + '</div></div>' +
                (req.error ? '<div class="modal-detail-row"><div class="modal-detail-label">Error</div><div class="modal-detail-value" style="color: var(--accent-red);">' + req.error + '</div></div>' : '');
            
            modal.style.display = 'flex';
        }

        function closeModal() {
            document.getElementById('requestModal').style.display = 'none';
        }


        // Pagination handlers
        document.getElementById('prevPage').addEventListener('click', () => {
            if (currentPage > 1) {
                currentPage--;
                renderTable();
            }
        });

        document.getElementById('nextPage').addEventListener('click', () => {
            const rows = currentTab === 'inflight' ? Object.values(requestsData.inFlight) : requestsData.recent;
            const totalPages = Math.ceil(rows.length / ITEMS_PER_PAGE);
            if (currentPage < totalPages) {
                currentPage++;
                renderTable();
            }
        });

        // Render errored requests
        function renderErroredRequests() {
            const tbody = document.getElementById('erroredRequestsBody');
            const errored = requestsData.recent.filter(req => 
                req.status && req.status !== 'success'
            ).slice(0, 10);

            if (errored.length === 0) {
                tbody.innerHTML = '<tr><td colspan="5" style="text-align: center; padding: 24px; color: var(--text-muted);">No errors yet</td></tr>';
                return;
            }

            tbody.innerHTML = errored.map(req => {
                const date = req.start_time ? new Date(req.start_time).toLocaleString() : '-';
                const endTime = req.last_activity_time || null;
                const duration = calculateDuration(req.start_time, endTime);
                const statusClass = getStatusClass(req.status);
                const statusText = getStatusText(req.status);
                
                // Store for modal
                requestDataMap.set(req.id, req);
                
                const statusTooltip = getStatusTooltip(req.status);
                return '<tr onclick="showRequestModal(\'' + req.id + '\')" style="cursor: pointer;">' +
                    '<td>' + date + '</td>' +
                    '<td>' + (req.model || '-') + '</td>' +
                    '<td><span class="status-badge ' + statusClass + '" data-tooltip="' + statusTooltip + '">' + statusText + '</span></td>' +
                    '<td>' + formatDuration(duration) + '</td>' +
                    '<td>' + (req.error || '-') + '</td>' +
                    '</tr>';
            }).join('');
        }

        // Format duration from seconds
        function formatDurationFromSeconds(seconds) {
            if (!seconds) return '-';
            if (seconds < 60) return seconds.toFixed(0) + 's';
            if (seconds < 3600) return (seconds / 60).toFixed(0) + 'm';
            return (seconds / 3600).toFixed(1) + 'h';
        }

        // Format config value
        function formatConfigValue(key, value) {
            if (value === null || value === undefined || value === '') {
                return '-';
            }
            if (typeof value === 'boolean') {
                return value ? 'true' : 'false';
            }
            if (Array.isArray(value)) {
                return value.length > 0 ? value.join(', ') : '[]';
            }
            return String(value);
        }

        // Store config data globally
        let configData = null;

        // Fetch configuration
        async function fetchConfig() {
            try {
                const response = await fetch('/config');
                if (!response.ok) {
                    throw new Error('Failed to fetch config');
                }
                configData = await response.json();
            } catch (error) {
                console.error('Failed to fetch config:', error);
                configData = null;
            }
        }

        // Show config modal
        function showConfigModal() {
            if (!configData) {
                fetchConfig().then(() => {
                    if (configData) {
                        renderConfigModal();
                        document.getElementById('configModal').style.display = 'flex';
                    }
                });
            } else {
                renderConfigModal();
                document.getElementById('configModal').style.display = 'flex';
            }
        }

        // Close config modal
        function closeConfigModal() {
            document.getElementById('configModal').style.display = 'none';
        }

        // Render config modal with tabs
        function renderConfigModal() {
            if (!configData) return;

            const categories = {
                'General': ['listen_addr', 'upstream_url', 'log_level', 'cors_allow_origin'],
                'Context': ['min_ctx', 'max_ctx', 'buckets', 'headroom', 'override_num_ctx'],
                'Output': ['default_output_budget', 'max_output_budget', 'structured_overhead', 'dynamic_default_output_budget'],
                'Estimation': ['default_fixed_overhead_tokens', 'default_per_message_overhead', 'default_tokens_per_byte', 'default_tokens_per_image_fallback'],
                'Performance': ['request_body_max_bytes', 'response_tap_max_bytes', 'show_cache_ttl', 'flush_interval'],
                'Calibration': ['calibration_enabled', 'calibration_file'],
                'Hardware': ['hardware_probe', 'hardware_probe_refresh', 'hardware_probe_vram_headroom'],
                'System': ['strip_system_prompt_text'],
                'Supervisor': ['supervisor_enabled', 'supervisor_track_requests', 'supervisor_recent_buffer'],
                'Watchdog': ['supervisor_watchdog_enabled', 'supervisor_ttfb_timeout', 'supervisor_stall_timeout', 'supervisor_hard_timeout'],
                'Observability': ['supervisor_obs_enabled', 'supervisor_obs_requests_endpoint', 'supervisor_obs_sse_endpoint', 'supervisor_obs_progress_interval'],
                'Loop Detection': ['supervisor_loop_detect_enabled', 'supervisor_loop_window_bytes', 'supervisor_loop_ngram_bytes', 'supervisor_loop_repeat_threshold', 'supervisor_loop_min_output_bytes'],
                'Retry': ['supervisor_retry_enabled', 'supervisor_retry_max_attempts', 'supervisor_retry_backoff', 'supervisor_retry_only_non_streaming', 'supervisor_retry_max_response_bytes'],
                'Restart': ['supervisor_restart_enabled', 'supervisor_restart_cmd', 'supervisor_restart_cooldown', 'supervisor_restart_max_per_hour', 'supervisor_restart_trigger_consec_timeouts', 'supervisor_restart_cmd_timeout'],
                'Metrics': ['supervisor_metrics_enabled', 'supervisor_metrics_path'],
                'Health': ['supervisor_health_check_enabled', 'supervisor_health_check_interval', 'supervisor_health_check_timeout'],
                'Safety': ['supervisor_output_safety_limit_enabled', 'supervisor_output_safety_limit_tokens', 'supervisor_output_safety_limit_action']
            };

            const configTabs = document.getElementById('configTabs');
            const configContent = document.getElementById('configContent');

            // Generate tabs
            let tabsHTML = '';
            let contentHTML = '';
            let firstTab = true;

            for (const [category, keys] of Object.entries(categories)) {
                const tabId = 'config-tab-' + category.toLowerCase().replace(/\s+/g, '-');
                tabsHTML += '<button class="config-tab' + (firstTab ? ' active' : '') + '" onclick="switchConfigTab(\'' + tabId + '\')" data-tab="' + tabId + '">' + category + '</button>';

                let listHTML = '<div class="config-list">';
                for (const key of keys) {
                    if (configData[key] !== undefined) {
                        const value = formatConfigValue(key, configData[key]);
                        const label = key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
                        listHTML += '<div class="config-list-item">';
                        listHTML += '<div class="config-list-label">' + label + '</div>';
                        listHTML += '<div class="config-list-value">' + value + '</div>';
                        listHTML += '</div>';
                    }
                }
                listHTML += '</div>';

                contentHTML += '<div class="config-content' + (firstTab ? ' active' : '') + '" id="' + tabId + '">' + listHTML + '</div>';
                firstTab = false;
            }

            configTabs.innerHTML = tabsHTML;
            configContent.innerHTML = contentHTML;
        }

        // Switch config tab
        function switchConfigTab(tabId) {
            // Update tab buttons
            document.querySelectorAll('.config-tab').forEach(tab => {
                tab.classList.remove('active');
                if (tab.dataset.tab === tabId) {
                    tab.classList.add('active');
                }
            });

            // Update content
            document.querySelectorAll('.config-content').forEach(content => {
                content.classList.remove('active');
                if (content.id === tabId) {
                    content.classList.add('active');
                }
            });
        }

        // Close modal on outside click
        window.onclick = function(event) {
            const requestModal = document.getElementById('requestModal');
            const configModal = document.getElementById('configModal');
            if (event.target === requestModal) {
                closeModal();
            }
            if (event.target === configModal) {
                closeConfigModal();
            }
        }

        // Tab switching
        document.querySelectorAll('.tab').forEach(tab => {
            tab.addEventListener('click', () => {
                document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
                tab.classList.add('active');
                currentTab = tab.dataset.tab;
                currentPage = 1; // Reset pagination on tab change
                renderTable();
            });
        });

        // Render 24-hour chart
        function renderHourlyChart() {
            const chartBars = document.getElementById('hourlyChart');
            
            if (!chartBars) return;
            
            const now = Date.now();
            const oneDayAgo = now - (24 * 60 * 60 * 1000);
            
            // Clean up old entries (older than 24 hours)
            hourlyRequests = hourlyRequests.filter(entry => entry.timestamp > oneDayAgo);
            
            // Create 24 buckets (one per hour)
            const buckets = new Array(24).fill(0);
            const bucketSize = 60 * 60 * 1000; // 1 hour in ms
            
            hourlyRequests.forEach(entry => {
                const age = now - entry.timestamp;
                const bucketIndex = Math.floor(age / bucketSize);
                if (bucketIndex >= 0 && bucketIndex < 24) {
                    buckets[23 - bucketIndex] += entry.count;
                }
            });
            
            const maxValue = Math.max(...buckets, 1);
            
            let barsHTML = '';
            let labelsHTML = '';
            
            for (let i = 0; i < 24; i++) {
                const hoursAgo = 23 - i;
                const hourTime = new Date(now - (hoursAgo * bucketSize));
                const hourLabel = hourTime.getHours() + 'h';
                const count = buckets[i];
                const height = maxValue > 0 ? (count / maxValue) * 100 : 0;
                
                barsHTML += '<div class="chart-bar-wrapper" style="flex: 1; display: flex; flex-direction: column; align-items: center; position: relative;">';
                barsHTML += '<div class="chart-bar" style="height: ' + height + '%; width: 100%;" title="' + count + ' requests ' + (hoursAgo === 0 ? 'now' : hoursAgo + 'h ago') + '"></div>';
                barsHTML += '<div class="chart-bar-label" style="margin-top: 8px; font-size: 10px; color: var(--text-muted);">' + (i % 4 === 0 || i === 23 ? hourLabel : '') + '</div>';
                barsHTML += '</div>';
            }
            
            chartBars.innerHTML = barsHTML;
        }

        // Track request by hour
        function trackRequestByHour(startTime) {
            if (!startTime) return;
            const timestamp = new Date(startTime).getTime();
            const now = Date.now();
            const oneDayAgo = now - (24 * 60 * 60 * 1000);
            
            // Only track if within last 24 hours
            if (timestamp < oneDayAgo) return;
            
            // Find existing entry for this hour (rounded to nearest hour)
            const hourTimestamp = Math.floor(timestamp / (60 * 60 * 1000)) * (60 * 60 * 1000);
            const existing = hourlyRequests.find(e => e.timestamp === hourTimestamp);
            
            if (existing) {
                existing.count++;
            } else {
                hourlyRequests.push({ timestamp: hourTimestamp, count: 1 });
            }
            
            renderHourlyChart();
        }

        // View toggle handler
        document.querySelectorAll('.view-toggle-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                document.querySelectorAll('.view-toggle-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                currentView = btn.dataset.view;
                currentPage = 1; // Reset pagination on view change
                renderTable();
            });
        });

        // Initial load
        fetchHealth();
        fetchRequests();
        fetchPrometheusMetrics();
        renderErroredRequests(); // Initialize errored requests
        fetchConfig(); // Load configuration
        renderHourlyChart(); // Initialize 24-hour chart
        connectSSE();

        // Refresh health every 10 seconds
        setInterval(fetchHealth, 10000);
        
        // Refresh metrics every 5 seconds
        setInterval(fetchPrometheusMetrics, 5000);
    </script>
</body>
</html>
`
