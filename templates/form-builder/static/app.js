const fieldTypeLabels = { text: '文本框', textarea: '多行文本', number: '数字', select: '下拉选择', checkbox: '复选框' };
let currentFields = [];
let currentPreviewId = null;

function loadForms() {
    fetch('/api/forms')
        .then(r => r.json())
        .then(forms => {
            const list = document.getElementById('formList');
            if (forms.length === 0) {
                list.innerHTML = '<div class="empty-state">暂无表单，点击上方按钮创建</div>';
                return;
            }
            list.innerHTML = '';
            forms.forEach(f => {
                const div = document.createElement('div');
                div.className = 'form-card';
                div.innerHTML = `
                    <h3>${escapeHtml(f.title)}</h3>
                    <p>${escapeHtml(f.description || '无描述')}</p>
                    <div class="card-meta">${f.fields.length} 个字段 | ${f.submission_count} 条提交</div>
                    <div class="card-actions">
                        <button onclick="previewForm(${f.id})">预览</button>
                        <button onclick="viewSubmissions(${f.id})">提交记录</button>
                        <button onclick="editForm(${f.id})">编辑</button>
                        <button class="btn-danger" onclick="deleteForm(${f.id})">删除</button>
                    </div>
                `;
                list.appendChild(div);
            });
        });
}

function showCreateForm() {
    document.getElementById('editorModal').style.display = 'flex';
    document.getElementById('editorTitle').textContent = '新建表单';
    document.getElementById('editFormId').value = '';
    document.getElementById('formTitleInput').value = '';
    document.getElementById('formDescInput').value = '';
    currentFields = [];
    renderFields();
}

function hideEditor() {
    document.getElementById('editorModal').style.display = 'none';
}

function addField() {
    const type = document.getElementById('newFieldType').value;
    currentFields.push({ label: '', type: type, options: '', required: false });
    renderFields();
}

function renderFields() {
    const container = document.getElementById('fieldsContainer');
    container.innerHTML = '';
    currentFields.forEach((f, i) => {
        const div = document.createElement('div');
        div.className = 'field-item';
        div.draggable = true;
        div.dataset.index = i;
        let optionsHtml = '';
        if (f.type === 'select') {
            optionsHtml = `<div class="field-options"><input type="text" placeholder="选项（逗号分隔）" value="${escapeAttr(f.options || '')}" onchange="updateFieldOptions(${i}, this.value)" /></div>`;
        }
        div.innerHTML = `
            <input type="text" placeholder="字段名称" value="${escapeAttr(f.label)}" onchange="updateFieldLabel(${i}, this.value)" />
            <select onchange="updateFieldType(${i}, this.value)">
                ${Object.entries(fieldTypeLabels).map(([k, v]) => `<option value="${k}" ${f.type === k ? 'selected' : ''}>${v}</option>`).join('')}
            </select>
            ${optionsHtml}
            <button class="remove-field" onclick="removeField(${i})">&times;</button>
        `;
        // Drag events for reordering
        div.addEventListener('dragstart', e => {
            e.dataTransfer.setData('text/plain', i);
            div.classList.add('dragging');
        });
        div.addEventListener('dragend', () => div.classList.remove('dragging'));
        div.addEventListener('dragover', e => e.preventDefault());
        div.addEventListener('drop', e => {
            e.preventDefault();
            const from = parseInt(e.dataTransfer.getData('text/plain'));
            const to = i;
            if (from !== to) {
                const item = currentFields.splice(from, 1)[0];
                currentFields.splice(to, 0, item);
                renderFields();
            }
        });
        container.appendChild(div);
    });
}

function updateFieldLabel(i, val) { currentFields[i].label = val; }
function updateFieldType(i, val) {
    currentFields[i].type = val;
    if (val !== 'select') currentFields[i].options = '';
    renderFields();
}
function updateFieldOptions(i, val) { currentFields[i].options = val; }
function removeField(i) { currentFields.splice(i, 1); renderFields(); }

function saveForm() {
    const id = document.getElementById('editFormId').value;
    const title = document.getElementById('formTitleInput').value.trim();
    if (!title) { alert('请输入表单标题'); return; }
    const data = {
        title,
        description: document.getElementById('formDescInput').value.trim(),
        fields: currentFields.map(f => ({
            label: f.label,
            type: f.type,
            options: f.type === 'select' ? (f.options || '').split(',').map(s => s.trim()).filter(Boolean) : []
        }))
    };
    const url = id ? `/api/forms/${id}` : '/api/forms';
    const method = id ? 'PUT' : 'POST';
    fetch(url, {
        method, headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    }).then(r => {
        if (r.ok) { hideEditor(); loadForms(); }
    });
}

function editForm(id) {
    fetch(`/api/forms/${id}`)
        .then(r => r.json())
        .then(f => {
            document.getElementById('editorModal').style.display = 'flex';
            document.getElementById('editorTitle').textContent = '编辑表单';
            document.getElementById('editFormId').value = f.id;
            document.getElementById('formTitleInput').value = f.title;
            document.getElementById('formDescInput').value = f.description || '';
            currentFields = f.fields.map(field => ({
                label: field.label,
                type: field.type,
                options: Array.isArray(field.options) ? field.options.join(', ') : ''
            }));
            renderFields();
        });
}

function deleteForm(id) {
    if (!confirm('确定删除此表单？')) return;
    fetch(`/api/forms/${id}`, { method: 'DELETE' })
        .then(() => loadForms());
}

function previewForm(id) {
    currentPreviewId = id;
    fetch(`/api/forms/${id}`)
        .then(r => r.json())
        .then(f => {
            document.getElementById('previewTitle').textContent = f.title;
            const form = document.getElementById('previewForm');
            form.innerHTML = '';
            f.fields.forEach((field, i) => {
                const wrap = document.createElement('div');
                wrap.className = 'preview-field';
                let inputHtml = '';
                const name = `field_${i}`;
                switch (field.type) {
                    case 'text':
                        inputHtml = `<input type="text" name="${name}" />`; break;
                    case 'textarea':
                        inputHtml = `<textarea name="${name}" rows="3"></textarea>`; break;
                    case 'number':
                        inputHtml = `<input type="number" name="${name}" />`; break;
                    case 'select':
                        const opts = (field.options || []).map(o => `<option value="${escapeAttr(o)}">${escapeHtml(o)}</option>`).join('');
                        inputHtml = `<select name="${name}"><option value="">请选择</option>${opts}</select>`; break;
                    case 'checkbox':
                        inputHtml = `<div class="checkbox-wrap"><input type="checkbox" name="${name}" /><span>是</span></div>`; break;
                }
                wrap.innerHTML = `<label>${escapeHtml(field.label || '未命名')}</label>${inputHtml}`;
                form.appendChild(wrap);
            });
            document.getElementById('previewModal').style.display = 'flex';
        });
}

function hidePreview() {
    document.getElementById('previewModal').style.display = 'none';
}

function submitPreview() {
    const form = document.getElementById('previewForm');
    const data = {};
    form.querySelectorAll('input, select, textarea').forEach(el => {
        if (el.type === 'checkbox') {
            data[el.name] = el.checked;
        } else {
            data[el.name] = el.value;
        }
    });
    fetch(`/api/forms/${currentPreviewId}/submit`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    }).then(r => {
        if (r.ok) {
            alert('提交成功');
            hidePreview();
            loadForms();
        }
    });
}

function viewSubmissions(id) {
    fetch(`/api/forms/${id}`)
        .then(r => r.json())
        .then(form => {
            fetch(`/api/forms/${id}/submissions`)
                .then(r => r.json())
                .then(subs => {
                    const list = document.getElementById('subsList');
                    if (subs.length === 0) {
                        list.innerHTML = '<div class="empty-state">暂无提交记录</div>';
                    } else {
                        list.innerHTML = '';
                        subs.forEach(s => {
                            const div = document.createElement('div');
                            div.className = 'sub-entry';
                            let fieldsHtml = '';
                            form.fields.forEach((field, i) => {
                                const key = `field_${i}`;
                                const val = s.data[key];
                                const display = typeof val === 'boolean' ? (val ? '是' : '否') : (val || '-');
                                fieldsHtml += `<div class="sub-field"><strong>${escapeHtml(field.label || '未命名')}：</strong>${escapeHtml(String(display))}</div>`;
                            });
                            div.innerHTML = `<div class="sub-time">${escapeHtml(s.submitted_at)}</div>${fieldsHtml}`;
                            list.appendChild(div);
                        });
                    }
                    document.getElementById('subsModal').style.display = 'flex';
                });
        });
}

function hideSubs() {
    document.getElementById('subsModal').style.display = 'none';
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function escapeAttr(str) {
    return str.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

loadForms();
