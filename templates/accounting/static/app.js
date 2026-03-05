const categoryLabels = {
    salary: '工资', freelance: '兼职', investment: '投资收益',
    gift: '礼金', other_income: '其他收入',
    food: '餐饮', transport: '交通', housing: '住房',
    entertainment: '娱乐', shopping: '购物', health: '医疗',
    education: '教育', other_expense: '其他支出'
};

function init() {
    document.getElementById('txDate').value = new Date().toISOString().slice(0, 10);
    loadTransactions();
    loadSummary();
}

function getFilterParams() {
    const params = new URLSearchParams();
    const dateFrom = document.getElementById('dateFrom').value;
    const dateTo = document.getElementById('dateTo').value;
    const category = document.getElementById('filterCategory').value;
    if (dateFrom) params.set('date_from', dateFrom);
    if (dateTo) params.set('date_to', dateTo);
    if (category) params.set('category', category);
    return params.toString();
}

function loadTransactions() {
    const qs = getFilterParams();
    fetch('/api/transactions' + (qs ? '?' + qs : ''))
        .then(r => r.json())
        .then(txs => {
            const tbody = document.getElementById('txBody');
            tbody.innerHTML = '';
            txs.forEach(tx => {
                const tr = document.createElement('tr');
                const typeLabel = tx.type === 'income' ? '收入' : '支出';
                const typeClass = tx.type === 'income' ? 'type-income' : 'type-expense';
                tr.innerHTML = `
                    <td>${escapeHtml(tx.date)}</td>
                    <td class="${typeClass}">${typeLabel}</td>
                    <td>${escapeHtml(categoryLabels[tx.category] || tx.category)}</td>
                    <td class="${typeClass}">${tx.type === 'expense' ? '-' : '+'}${Number(tx.amount).toFixed(2)}</td>
                    <td>${escapeHtml(tx.note || '')}</td>
                    <td><button class="delete-btn" onclick="deleteTx(${tx.id})">&times;</button></td>
                `;
                tbody.appendChild(tr);
            });
        });
}

function loadSummary() {
    const qs = getFilterParams();
    fetch('/api/summary' + (qs ? '?' + qs : ''))
        .then(r => r.json())
        .then(s => {
            document.getElementById('totalIncome').textContent = Number(s.income).toFixed(2);
            document.getElementById('totalExpense').textContent = Number(s.expense).toFixed(2);
            document.getElementById('totalBalance').textContent = Number(s.balance).toFixed(2);
        });
}

function addTransaction() {
    const date = document.getElementById('txDate').value;
    const amount = document.getElementById('txAmount').value;
    const category = document.getElementById('txCategory').value;
    const note = document.getElementById('txNote').value;
    if (!amount || Number(amount) <= 0) { alert('请输入正确的金额'); return; }
    fetch('/api/transactions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ date, amount: Number(amount), category, note })
    }).then(r => {
        if (r.ok) {
            document.getElementById('txAmount').value = '';
            document.getElementById('txNote').value = '';
            loadTransactions();
            loadSummary();
        }
    });
}

function deleteTx(id) {
    fetch(`/api/transactions/${id}`, { method: 'DELETE' })
        .then(() => { loadTransactions(); loadSummary(); });
}

function applyFilter() {
    loadTransactions();
    loadSummary();
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

init();
