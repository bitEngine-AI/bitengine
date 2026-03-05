const statusLabels = { lead: '潜在客户', customer: '正式客户', churned: '已流失' };

function loadContacts() {
    const search = document.getElementById('searchInput').value;
    const status = document.getElementById('statusFilter').value;
    const params = new URLSearchParams();
    if (search) params.set('search', search);
    if (status) params.set('status', status);
    const qs = params.toString();

    fetch('/api/contacts' + (qs ? '?' + qs : ''))
        .then(r => r.json())
        .then(contacts => {
            const tbody = document.getElementById('contactsBody');
            tbody.innerHTML = '';
            contacts.forEach(c => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td>${escapeHtml(c.name)}</td>
                    <td>${escapeHtml(c.email)}</td>
                    <td>${escapeHtml(c.phone)}</td>
                    <td>${escapeHtml(c.company)}</td>
                    <td><span class="status-badge status-${c.status}">${statusLabels[c.status] || c.status}</span></td>
                    <td>
                        <button class="action-btn" onclick="editContact(${c.id})">编辑</button>
                        <button class="action-btn delete" onclick="deleteContact(${c.id})">删除</button>
                    </td>
                `;
                tbody.appendChild(tr);
            });
        });
}

function showForm(contact) {
    document.getElementById('contactModal').style.display = 'flex';
    if (contact) {
        document.getElementById('formTitle').textContent = '编辑客户';
        document.getElementById('editId').value = contact.id;
        document.getElementById('fName').value = contact.name;
        document.getElementById('fEmail').value = contact.email;
        document.getElementById('fPhone').value = contact.phone;
        document.getElementById('fCompany').value = contact.company;
        document.getElementById('fStatus').value = contact.status;
        document.getElementById('fNotes').value = contact.notes;
    } else {
        document.getElementById('formTitle').textContent = '新建客户';
        document.getElementById('editId').value = '';
        document.getElementById('fName').value = '';
        document.getElementById('fEmail').value = '';
        document.getElementById('fPhone').value = '';
        document.getElementById('fCompany').value = '';
        document.getElementById('fStatus').value = 'lead';
        document.getElementById('fNotes').value = '';
    }
}

function hideForm() {
    document.getElementById('contactModal').style.display = 'none';
}

function saveContact() {
    const id = document.getElementById('editId').value;
    const data = {
        name: document.getElementById('fName').value.trim(),
        email: document.getElementById('fEmail').value.trim(),
        phone: document.getElementById('fPhone').value.trim(),
        company: document.getElementById('fCompany').value.trim(),
        status: document.getElementById('fStatus').value,
        notes: document.getElementById('fNotes').value.trim()
    };
    if (!data.name) { alert('请输入姓名'); return; }

    const url = id ? `/api/contacts/${id}` : '/api/contacts';
    const method = id ? 'PUT' : 'POST';
    fetch(url, {
        method, headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    }).then(r => {
        if (r.ok) { hideForm(); loadContacts(); }
    });
}

function editContact(id) {
    fetch(`/api/contacts/${id}`)
        .then(r => r.json())
        .then(c => showForm(c));
}

function deleteContact(id) {
    if (!confirm('确定删除此客户？')) return;
    fetch(`/api/contacts/${id}`, { method: 'DELETE' })
        .then(() => loadContacts());
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str || '';
    return div.innerHTML;
}

loadContacts();
