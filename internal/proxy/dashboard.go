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
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>ollama-auto-ctx</h1>
            <div class="health-badge">
                <div class="health-dot" id="healthDot"></div>
                <span id="healthText">Checking...</span>
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

        <div class="requests-section">
            <div class="section-header">
                <div class="section-title">Request Monitor</div>
                <div class="section-subtitle">Real-time request tracking and analytics</div>
            </div>

            <div class="tabs">
                <button class="tab active" data-tab="inflight">In-Flight</button>
                <button class="tab" data-tab="recent">Recent</button>
            </div>

            <div id="errorMessage" class="error" style="display: none;"></div>

            <div class="table-container">
                <div id="loadingState" class="loading">Loading requests...</div>
                <table id="requestsTable" style="display: none;">
                    <thead>
                        <tr>
                            <th>ID</th>
                            <th>Model</th>
                            <th>Endpoint</th>
                            <th>Status</th>
                            <th>Duration</th>
                            <th>Bytes</th>
                            <th>Tokens</th>
                        </tr>
                    </thead>
                    <tbody id="tableBody"></tbody>
                </table>
                <div id="emptyState" class="empty-state" style="display: none;">
                    No requests to display
                </div>
            </div>
        </div>

        <div class="metrics-section">
            <div class="section-header">
                <div class="section-title">Session Statistics</div>
                <div class="section-subtitle">In-memory stats since page load (resets on refresh)</div>
            </div>

            <div class="metrics-grid-large" id="sessionMetrics">
                <div class="metric-item">
                    <div class="metric-item-label">Requests (Session)</div>
                    <div class="metric-item-value" id="sessionRequestCount">0</div>
                    <div class="metric-item-sub">Since page load</div>
                </div>
                <div class="metric-item">
                    <div class="metric-item-label">Avg Completion Time</div>
                    <div class="metric-item-value" id="avgCompletionTime">-</div>
                    <div class="metric-item-sub">Completed requests</div>
                </div>
                <div class="metric-item">
                    <div class="metric-item-label">Success</div>
                    <div class="metric-item-value" id="sessionSuccessCount" style="color: var(--accent-green);">0</div>
                    <div class="metric-item-sub">Successful requests</div>
                </div>
                <div class="metric-item">
                    <div class="metric-item-label">Failures</div>
                    <div class="metric-item-value" id="sessionFailureCount" style="color: var(--accent-red);">0</div>
                    <div class="metric-item-sub">Errors & timeouts</div>
                </div>
                <div class="metric-item">
                    <div class="metric-item-label">Success Rate</div>
                    <div class="metric-item-value" id="sessionSuccessRate">-</div>
                    <div class="metric-item-sub">Session percentage</div>
                </div>
                <div class="metric-item">
                    <div class="metric-item-label">Timeouts</div>
                    <div class="metric-item-value" id="sessionTimeoutCount" style="color: var(--accent-yellow);">0</div>
                    <div class="metric-item-sub">All timeout types</div>
                </div>
            </div>
        </div>

        <div class="metrics-section">
            <div class="section-header">
                <div class="section-title">Recent Requests</div>
                <div class="section-subtitle">Last 10 completed requests with context and timing data</div>
            </div>

            <div class="table-container">
                <table id="recentRequestsTable">
                    <thead>
                        <tr>
                            <th>Model</th>
                            <th>Context</th>
                            <th>Duration</th>
                            <th>Status</th>
                        </tr>
                    </thead>
                    <tbody id="recentRequestsBody">
                        <tr>
                            <td colspan="4" style="text-align: center; padding: 24px; color: var(--text-muted);">No completed requests yet</td>
                        </tr>
                    </tbody>
                </table>
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
    </div>

    <script>
        let currentTab = 'inflight';
        let requestsData = { inFlight: {}, recent: [] };
        let eventSource = null;

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

        // Track request start
        function trackRequestStart() {
            sessionStats.totalRequests++;
            updateSessionMetrics();
        }

        // Track request completion
        function trackRequestCompletion(status, duration, requestId, model, contextSize) {
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
                    context: contextSize || null,
                    duration: duration,
                    status: status
                });
            }
            
            updateSessionMetrics();
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
            
            renderRecentRequestsTable();
        }

        // Render recent requests table
        function renderRecentRequestsTable() {
            const tbody = document.getElementById('recentRequestsBody');
            
            if (recentCompletedRequests.length === 0) {
                tbody.innerHTML = '<tr><td colspan="4" style="text-align: center; padding: 24px; color: var(--text-muted);">No completed requests yet</td></tr>';
                return;
            }
            
            tbody.innerHTML = recentCompletedRequests.map(req => {
                const statusClass = getStatusClass(req.status);
                const statusText = getStatusText(req.status);
                const contextDisplay = req.context ? formatNumber(req.context) : '-';
                
                return '<tr>' +
                    '<td>' + (req.model || '-') + '</td>' +
                    '<td>' + contextDisplay + '</td>' +
                    '<td>' + formatDuration(req.duration) + '</td>' +
                    '<td><span class="status-badge ' + statusClass + '">' + statusText + '</span></td>' +
                    '</tr>';
            }).join('');
        }

        // Update session metrics display
        function updateSessionMetrics() {
            document.getElementById('sessionRequestCount').textContent = formatNumber(sessionStats.totalRequests);
            document.getElementById('sessionSuccessCount').textContent = formatNumber(sessionStats.successCount);
            document.getElementById('sessionFailureCount').textContent = formatNumber(sessionStats.failureCount);
            document.getElementById('sessionTimeoutCount').textContent = formatNumber(sessionStats.timeoutCount);
            
            // Calculate average completion time
            if (sessionStats.completionTimes.length > 0) {
                const avgTime = sessionStats.completionTimes.reduce((a, b) => a + b, 0) / sessionStats.completionTimes.length;
                document.getElementById('avgCompletionTime').textContent = formatDuration(avgTime);
            } else {
                document.getElementById('avgCompletionTime').textContent = '-';
            }
            
            // Calculate success rate
            if (sessionStats.completedRequests > 0) {
                const successRate = ((sessionStats.successCount / sessionStats.completedRequests) * 100).toFixed(1);
                document.getElementById('sessionSuccessRate').textContent = successRate + '%';
            } else {
                document.getElementById('sessionSuccessRate').textContent = '-';
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

            loadingState.style.display = 'none';

            let rows = [];
            if (currentTab === 'inflight') {
                rows = Object.values(requestsData.inFlight);
            } else {
                rows = requestsData.recent;
            }

            if (rows.length === 0) {
                table.style.display = 'none';
                emptyState.style.display = 'block';
                return;
            }

            table.style.display = 'table';
            emptyState.style.display = 'none';

            tbody.innerHTML = rows.map(req => {
                const duration = calculateDuration(req.start_time);
                const status = req.status || '';
                const statusClass = getStatusClass(status);
                const statusText = getStatusText(status);
                return '<tr>' +
                    '<td><code>' + req.id + '</code></td>' +
                    '<td>' + (req.model || '-') + '</td>' +
                    '<td>' + (req.endpoint || '-') + '</td>' +
                    '<td><span class="status-badge ' + statusClass + '">' + statusText + '</span></td>' +
                    '<td>' + formatDuration(duration) + '</td>' +
                    '<td>' + formatBytes(req.bytes_forwarded || 0) + '</td>' +
                    '<td>' + formatNumber(req.estimated_output_tokens || 0) + '</td>' +
                    '</tr>';
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
                
                // Only track if request has a status (completed)
                if (req.status) {
                    let duration = null;
                    if (req.start_time) {
                        const startTime = new Date(req.start_time);
                        // Use last_activity_time or current time as end time
                        const endTime = req.last_activity_time ? new Date(req.last_activity_time) : new Date();
                        duration = endTime - startTime;
                    }
                    
                    // Context size is not available in RequestInfo, so we'll pass null
                    // This could be enhanced later if context data is added to tracking
                    trackRequestCompletion(req.status, duration, req.id, req.model, null);
                }
            }
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
                
                // Average duration (simplified - using histogram sum)
                const durationSum = sumMetric(metrics['ollama_proxy_request_duration_seconds']);
                const durationCount = metrics['ollama_proxy_request_duration_seconds']?.length || 1;
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
            }
            
            // Track request completion
            if (event.type === 'done' || event.type === 'canceled' || 
                event.type === 'timeout_ttfb' || event.type === 'timeout_stall' || 
                event.type === 'timeout_hard' || event.type === 'upstream_error' || 
                event.type === 'loop_detected' || event.type === 'output_limit_exceeded') {
                
                // Calculate duration and get request details
                let duration = null;
                let model = null;
                let contextSize = null;
                
                if (event.request_id) {
                    // Try to get details from in-flight request
                    const req = requestsData.inFlight[event.request_id];
                    if (req) {
                        model = req.model || event.model || null;
                        // Context size is not available in RequestInfo currently
                        // This could be enhanced if context data is added to tracking
                        contextSize = null;
                        
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
                trackRequestCompletion(status, duration, event.request_id, model, contextSize);
            }
            
            // Refresh requests data on significant events
            if (event.type === 'request_start' || event.type === 'done' || 
                event.type === 'canceled' || event.type.startsWith('timeout') ||
                event.type === 'upstream_error' || event.type === 'loop_detected') {
                fetchRequests();
            } else if (event.type === 'progress' || event.type === 'first_byte') {
                // Update in-flight request if it exists
                if (requestsData.inFlight[event.request_id]) {
                    const req = requestsData.inFlight[event.request_id];
                    if (event.bytes_out !== undefined) req.bytes_forwarded = event.bytes_out;
                    if (event.estimated_output_tokens !== undefined) req.estimated_output_tokens = event.estimated_output_tokens;
                    if (event.type === 'first_byte' && event.ttfb_ms !== undefined) {
                        req.first_byte_time = new Date(Date.now() - event.ttfb_ms).toISOString();
                    }
                    updateMetrics();
                    renderTable();
                }
            }
        }

        // Tab switching
        document.querySelectorAll('.tab').forEach(tab => {
            tab.addEventListener('click', () => {
                document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
                tab.classList.add('active');
                currentTab = tab.dataset.tab;
                renderTable();
            });
        });

        // Initial load
        fetchHealth();
        fetchRequests();
        fetchPrometheusMetrics();
        updateSessionMetrics(); // Initialize session metrics display
        renderRecentRequestsTable(); // Initialize recent requests table
        connectSSE();

        // Refresh health every 10 seconds
        setInterval(fetchHealth, 10000);
        
        // Refresh metrics every 5 seconds
        setInterval(fetchPrometheusMetrics, 5000);
    </script>
</body>
</html>
`
