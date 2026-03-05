function loadKPI() {
    fetch('/api/kpi')
        .then(r => r.json())
        .then(data => {
            document.getElementById('kpiSales').textContent = formatMoney(data.total_sales);
            document.getElementById('kpiVisitors').textContent = formatNumber(data.total_visitors);
            document.getElementById('kpiOrders').textContent = formatNumber(data.total_orders);
            document.getElementById('kpiConversion').textContent = data.avg_conversion.toFixed(2) + '%';
        });
}

function loadChart() {
    fetch('/api/chart')
        .then(r => r.json())
        .then(data => {
            const chart = document.getElementById('barChart');
            chart.innerHTML = '';
            if (data.length === 0) return;
            const maxSales = Math.max(...data.map(d => d.sales));
            data.forEach(d => {
                const wrapper = document.createElement('div');
                wrapper.className = 'bar-wrapper';
                const height = maxSales > 0 ? (d.sales / maxSales * 180) : 2;
                const value = document.createElement('div');
                value.className = 'bar-value';
                value.textContent = formatCompact(d.sales);
                const bar = document.createElement('div');
                bar.className = 'bar';
                bar.style.height = height + 'px';
                bar.title = `${d.date}: ${formatMoney(d.sales)}`;
                const label = document.createElement('div');
                label.className = 'bar-label';
                label.textContent = d.date.slice(5); // MM-DD
                wrapper.appendChild(value);
                wrapper.appendChild(bar);
                wrapper.appendChild(label);
                chart.appendChild(wrapper);
            });
        });
}

function loadTable() {
    fetch('/api/table')
        .then(r => r.json())
        .then(data => {
            const tbody = document.getElementById('tableBody');
            tbody.innerHTML = '';
            data.forEach(d => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td>${d.date}</td>
                    <td>${formatMoney(d.sales)}</td>
                    <td>${formatNumber(d.visitors)}</td>
                    <td>${formatNumber(d.orders)}</td>
                    <td>${d.conversion_rate.toFixed(2)}%</td>
                `;
                tbody.appendChild(tr);
            });
        });
}

function formatMoney(n) {
    return new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY' }).format(n);
}

function formatNumber(n) {
    return new Intl.NumberFormat('zh-CN').format(n);
}

function formatCompact(n) {
    if (n >= 10000) return (n / 10000).toFixed(1) + 'w';
    if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
    return Math.round(n).toString();
}

loadKPI();
loadChart();
loadTable();
