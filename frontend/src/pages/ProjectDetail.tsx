import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import api from '@/lib/api';
import { useAuth } from '@/hooks/useAuth';
import type { Task, ProjectWithTasks, User } from '@/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Select } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import {
  Plus, ArrowLeft, Pencil, Trash2, AlertCircle,
  CheckCircle2, Clock, Circle, ListTodo, UserCircle,
} from 'lucide-react';
import { formatDate } from '@/lib/utils';

const taskSchema = z.object({
  title: z.string().min(1, 'Title is required'),
  description: z.string().optional(),
  priority: z.enum(['low', 'medium', 'high']),
  status: z.enum(['todo', 'in_progress', 'done']).optional(),
  assignee_id: z.string().optional(),
  due_date: z.string().optional(),
});

type TaskForm = z.infer<typeof taskSchema>;

const editProjectSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  description: z.string().optional(),
});

type EditProjectForm = z.infer<typeof editProjectSchema>;

const statusConfig = {
  todo: { label: 'To Do', icon: Circle, variant: 'secondary' as const },
  in_progress: { label: 'In Progress', icon: Clock, variant: 'warning' as const },
  done: { label: 'Done', icon: CheckCircle2, variant: 'success' as const },
};

const priorityConfig = {
  low: { label: 'Low', variant: 'secondary' as const },
  medium: { label: 'Medium', variant: 'outline' as const },
  high: { label: 'High', variant: 'destructive' as const },
};

export function ProjectDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const [taskDialogOpen, setTaskDialogOpen] = useState(false);
  const [editProjectOpen, setEditProjectOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<Task | null>(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [assigneeFilter, setAssigneeFilter] = useState('');
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const { data: project, isLoading, isError, refetch } = useQuery({
    queryKey: ['project', id],
    queryFn: async () => {
      const { data } = await api.get<ProjectWithTasks>(`/projects/${id}`);
      return data;
    },
  });

  // Fetch all registered users for the assignee picker
  const { data: members = [] } = useQuery({
    queryKey: ['all-users'],
    queryFn: async () => {
      const { data } = await api.get<{ users: User[] }>('/users');
      return data.users ?? [];
    },
  });

  const isOwner = project?.owner_id === user?.id;

  const updateProjectMutation = useMutation({
    mutationFn: (body: EditProjectForm) => api.patch(`/projects/${id}`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project', id] });
      setEditProjectOpen(false);
    },
  });

  const deleteProjectMutation = useMutation({
    mutationFn: () => api.delete(`/projects/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      navigate('/projects');
    },
  });

  const createTaskMutation = useMutation({
    mutationFn: (body: TaskForm) => api.post(`/projects/${id}/tasks`, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project', id] });
      setTaskDialogOpen(false);
      taskForm.reset();
    },
  });

  const updateTaskMutation = useMutation({
    mutationFn: ({ taskId, body }: { taskId: string; body: Partial<TaskForm> }) =>
      api.patch(`/tasks/${taskId}`, body),
    onMutate: async ({ taskId, body }) => {
      await queryClient.cancelQueries({ queryKey: ['project', id] });
      const prev = queryClient.getQueryData<ProjectWithTasks>(['project', id]);
      if (prev) {
        queryClient.setQueryData<ProjectWithTasks>(['project', id], {
          ...prev,
          tasks: prev.tasks.map((t) => (t.id === taskId ? { ...t, ...body } : t)),
        });
      }
      return { prev };
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) {
        queryClient.setQueryData(['project', id], context.prev);
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['project', id] });
      setEditingTask(null);
      setTaskDialogOpen(false);
    },
  });

  const deleteTaskMutation = useMutation({
    mutationFn: (taskId: string) => api.delete(`/tasks/${taskId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project', id] });
      setDeleteConfirm(null);
    },
  });

  const taskForm = useForm<TaskForm>({
    resolver: zodResolver(taskSchema),
    defaultValues: { priority: 'medium', status: 'todo' },
  });

  const editProjectForm = useForm<EditProjectForm>({
    resolver: zodResolver(editProjectSchema),
  });

  const openCreateTask = () => {
    setEditingTask(null);
    taskForm.reset({ title: '', description: '', priority: 'medium', status: 'todo', assignee_id: '', due_date: '' });
    setTaskDialogOpen(true);
  };

  const openEditTask = (task: Task) => {
    setEditingTask(task);
    taskForm.reset({
      title: task.title,
      description: task.description,
      priority: task.priority,
      status: task.status,
      assignee_id: task.assignee_id || '',
      due_date: task.due_date || '',
    });
    setTaskDialogOpen(true);
  };

  const openEditProject = () => {
    if (project) {
      editProjectForm.reset({ name: project.name, description: project.description });
      setEditProjectOpen(true);
    }
  };

  const onTaskSubmit = (data: TaskForm) => {
    const cleaned = {
      ...data,
      assignee_id: data.assignee_id || undefined,
      due_date: data.due_date || undefined,
    };
    if (editingTask) {
      updateTaskMutation.mutate({ taskId: editingTask.id, body: cleaned });
    } else {
      createTaskMutation.mutate(cleaned);
    }
  };

  const cycleStatus = (task: Task) => {
    const order: Task['status'][] = ['todo', 'in_progress', 'done'];
    const next = order[(order.indexOf(task.status) + 1) % order.length];
    updateTaskMutation.mutate({ taskId: task.id, body: { status: next } });
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-4 w-96" />
        <div className="space-y-3">
          {[1, 2, 3].map((i) => <Skeleton key={i} className="h-20 rounded-lg" />)}
        </div>
      </div>
    );
  }

  if (isError || !project) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <AlertCircle className="h-12 w-12 text-destructive mb-4" />
        <h2 className="text-lg font-semibold">Failed to load project</h2>
        <p className="text-sm text-muted-foreground mb-4">The project may not exist or you may not have access.</p>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => navigate('/projects')}>Back to Projects</Button>
          <Button onClick={() => refetch()}>Retry</Button>
        </div>
      </div>
    );
  }

  const filteredTasks = (project.tasks || []).filter((t) => {
    if (statusFilter && t.status !== statusFilter) return false;
    if (assigneeFilter) {
      if (assigneeFilter === 'unassigned') return t.assignee_id === null;
      return t.assignee_id === assigneeFilter;
    }
    return true;
  });

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <button
            onClick={() => navigate('/projects')}
            className="mb-2 flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="h-4 w-4" /> Back to Projects
          </button>
          <h1 className="text-2xl font-bold truncate">{project.name}</h1>
          {project.description && (
            <p className="mt-1 text-muted-foreground">{project.description}</p>
          )}
        </div>
        {isOwner && (
          <div className="flex shrink-0 gap-2">
            <Button variant="outline" size="sm" onClick={openEditProject}>
              <Pencil className="mr-1 h-3.5 w-3.5" /> Edit
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={() => setDeleteConfirm('project')}
            >
              <Trash2 className="mr-1 h-3.5 w-3.5" /> Delete
            </Button>
          </div>
        )}
      </div>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-wrap items-center gap-2">
          <Select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="w-40"
          >
            <option value="">All statuses</option>
            <option value="todo">To Do</option>
            <option value="in_progress">In Progress</option>
            <option value="done">Done</option>
          </Select>
          <Select
            value={assigneeFilter}
            onChange={(e) => setAssigneeFilter(e.target.value)}
            className="w-44"
          >
            <option value="">All assignees</option>
            <option value="unassigned">Unassigned</option>
            {members.map((m) => (
              <option key={m.id} value={m.id}>{m.name}</option>
            ))}
          </Select>
        </div>
        <Button onClick={openCreateTask}>
          <Plus className="mr-2 h-4 w-4" /> Add Task
        </Button>
      </div>

      {filteredTasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border py-16 text-center">
          <ListTodo className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-lg font-semibold">No tasks found</h2>
          <p className="text-sm text-muted-foreground mb-4">
            {statusFilter ? 'No tasks match the current filter.' : 'Create your first task for this project.'}
          </p>
          {!statusFilter && (
            <Button onClick={openCreateTask}>
              <Plus className="mr-2 h-4 w-4" /> Add Task
            </Button>
          )}
        </div>
      ) : (
        <div className="space-y-3">
          {filteredTasks.map((task) => {
            const sc = statusConfig[task.status];
            const pc = priorityConfig[task.priority];
            const StatusIcon = sc.icon;
            return (
              <div
                key={task.id}
                className="group flex items-start gap-3 rounded-lg border border-border p-4 transition-colors hover:bg-accent/30"
              >
                <button
                  onClick={() => cycleStatus(task)}
                  className="mt-0.5 shrink-0 text-muted-foreground hover:text-foreground transition-colors"
                  title={`Status: ${sc.label} (click to cycle)`}
                >
                  <StatusIcon className="h-5 w-5" />
                </button>

                <div className="min-w-0 flex-1">
                  <div className="flex items-start justify-between gap-2">
                    <h3 className={`font-medium ${task.status === 'done' ? 'line-through text-muted-foreground' : ''}`}>
                      {task.title}
                    </h3>
                    <div className="flex shrink-0 items-center gap-1">
                      <Badge variant={sc.variant}>{sc.label}</Badge>
                      <Badge variant={pc.variant}>{pc.label}</Badge>
                    </div>
                  </div>
                  {task.description && (
                    <p className="mt-1 text-sm text-muted-foreground line-clamp-2">{task.description}</p>
                  )}
                  <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                    {task.assignee_id ? (
                      <span className="flex items-center gap-1">
                        <UserCircle className="h-3.5 w-3.5" />
                        {members.find((m) => m.id === task.assignee_id)?.name ?? 'Assigned'}
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 text-muted-foreground/60">
                        <UserCircle className="h-3.5 w-3.5" />
                        Unassigned
                      </span>
                    )}
                    {task.due_date && <span>Due {formatDate(task.due_date)}</span>}
                    <span>Created {formatDate(task.created_at)}</span>
                  </div>
                </div>

                <div className="flex shrink-0 gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <Button variant="ghost" size="icon" onClick={() => openEditTask(task)}>
                    <Pencil className="h-3.5 w-3.5" />
                  </Button>
                  <Button variant="ghost" size="icon" onClick={() => setDeleteConfirm(task.id)}>
                    <Trash2 className="h-3.5 w-3.5 text-destructive" />
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Task Create/Edit Dialog */}
      <Dialog
        open={taskDialogOpen}
        onClose={() => { setTaskDialogOpen(false); setEditingTask(null); taskForm.reset(); }}
      >
        <DialogHeader>
          <DialogTitle>{editingTask ? 'Edit Task' : 'Create Task'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={taskForm.handleSubmit(onTaskSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="task-title">Title</Label>
            <Input id="task-title" placeholder="Task title" {...taskForm.register('title')} />
            {taskForm.formState.errors.title && (
              <p className="text-sm text-destructive">{taskForm.formState.errors.title.message}</p>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="task-desc">Description</Label>
            <Textarea id="task-desc" placeholder="Optional description" {...taskForm.register('description')} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="task-priority">Priority</Label>
              <Select id="task-priority" {...taskForm.register('priority')}>
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
              </Select>
            </div>
            {editingTask && (
              <div className="space-y-2">
                <Label htmlFor="task-status">Status</Label>
                <Select id="task-status" {...taskForm.register('status')}>
                  <option value="todo">To Do</option>
                  <option value="in_progress">In Progress</option>
                  <option value="done">Done</option>
                </Select>
              </div>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="task-assignee">Assignee</Label>
            <Select id="task-assignee" {...taskForm.register('assignee_id')}>
              <option value="">Unassigned</option>
              {members.map((m) => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="task-due">Due Date</Label>
            <Input id="task-due" type="date" {...taskForm.register('due_date')} />
          </div>
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => { setTaskDialogOpen(false); setEditingTask(null); taskForm.reset(); }}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={createTaskMutation.isPending || updateTaskMutation.isPending}>
              {editingTask ? 'Save Changes' : 'Create Task'}
            </Button>
          </div>
        </form>
      </Dialog>

      {/* Edit Project Dialog */}
      <Dialog open={editProjectOpen} onClose={() => setEditProjectOpen(false)}>
        <DialogHeader>
          <DialogTitle>Edit Project</DialogTitle>
        </DialogHeader>
        <form
          onSubmit={editProjectForm.handleSubmit((data) => updateProjectMutation.mutate(data))}
          className="space-y-4"
        >
          <div className="space-y-2">
            <Label htmlFor="edit-name">Name</Label>
            <Input id="edit-name" {...editProjectForm.register('name')} />
            {editProjectForm.formState.errors.name && (
              <p className="text-sm text-destructive">{editProjectForm.formState.errors.name.message}</p>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="edit-desc">Description</Label>
            <Textarea id="edit-desc" {...editProjectForm.register('description')} />
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => setEditProjectOpen(false)}>Cancel</Button>
            <Button type="submit" disabled={updateProjectMutation.isPending}>Save Changes</Button>
          </div>
        </form>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={!!deleteConfirm} onClose={() => setDeleteConfirm(null)}>
        <DialogHeader>
          <DialogTitle>Confirm Delete</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-muted-foreground mb-4">
          {deleteConfirm === 'project'
            ? 'This will permanently delete this project and all its tasks. This action cannot be undone.'
            : 'This will permanently delete this task. This action cannot be undone.'}
        </p>
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={() => setDeleteConfirm(null)}>Cancel</Button>
          <Button
            variant="destructive"
            onClick={() => {
              if (deleteConfirm === 'project') {
                deleteProjectMutation.mutate();
              } else if (deleteConfirm) {
                deleteTaskMutation.mutate(deleteConfirm);
              }
            }}
            disabled={deleteProjectMutation.isPending || deleteTaskMutation.isPending}
          >
            Delete
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
