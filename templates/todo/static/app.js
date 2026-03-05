let draggedId = null;

function loadTasks() {
    fetch('/api/tasks')
        .then(r => r.json())
        .then(tasks => {
            document.getElementById('col-todo').innerHTML = '';
            document.getElementById('col-in_progress').innerHTML = '';
            document.getElementById('col-done').innerHTML = '';
            tasks.forEach(t => {
                const col = document.getElementById('col-' + t.status);
                if (col) col.appendChild(createCard(t));
            });
        });
}

function createCard(task) {
    const div = document.createElement('div');
    div.className = 'task-card';
    div.draggable = true;
    div.dataset.id = task.id;
    const priorityLabels = { high: '高', medium: '中', low: '低' };
    div.innerHTML = `
        <div class="task-title">${escapeHtml(task.title)}</div>
        <div class="task-meta">
            <span class="priority-badge priority-${task.priority}">${priorityLabels[task.priority] || task.priority}</span>
            <button class="delete-btn" onclick="deleteTask(${task.id})">&times;</button>
        </div>
    `;
    div.addEventListener('dragstart', e => {
        draggedId = task.id;
        div.classList.add('dragging');
        e.dataTransfer.effectAllowed = 'move';
    });
    div.addEventListener('dragend', () => {
        div.classList.remove('dragging');
    });
    return div;
}

function onDragOver(e) {
    e.preventDefault();
    e.currentTarget.classList.add('drag-over');
}

function onDrop(e) {
    e.preventDefault();
    e.currentTarget.classList.remove('drag-over');
    const status = e.currentTarget.dataset.status;
    if (draggedId && status) {
        fetch(`/api/tasks/${draggedId}/status`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ status: status })
        }).then(() => loadTasks());
    }
    draggedId = null;
}

// Also remove drag-over when leaving
document.querySelectorAll('.column').forEach(col => {
    col.addEventListener('dragleave', () => col.classList.remove('drag-over'));
});

function addTask() {
    const title = document.getElementById('taskTitle').value.trim();
    if (!title) return;
    const priority = document.getElementById('taskPriority').value;
    fetch('/api/tasks', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, priority })
    }).then(r => {
        if (r.ok) {
            document.getElementById('taskTitle').value = '';
            loadTasks();
        }
    });
}

function deleteTask(id) {
    fetch(`/api/tasks/${id}`, { method: 'DELETE' })
        .then(() => loadTasks());
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

document.getElementById('taskTitle').addEventListener('keydown', e => {
    if (e.key === 'Enter') addTask();
});

loadTasks();
