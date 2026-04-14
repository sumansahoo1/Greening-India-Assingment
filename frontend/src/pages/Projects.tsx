import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import api from '@/lib/api';
import type { Project } from '@/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Plus, FolderOpen, AlertCircle } from 'lucide-react';
import { formatDate } from '@/lib/utils';
import { usePreferences, useUpdatePreferences } from '@/hooks/usePreferences';

const createSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  description: z.string().optional(),
});

type CreateForm = z.infer<typeof createSchema>;

export function ProjectsPage() {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [page, setPage] = useState(1);
  const { data: prefs, isLoading: prefsLoading, isError: prefsError } = usePreferences();
  const updatePrefs = useUpdatePreferences();
  const limit = prefs?.projects_page_size;
  const queryClient = useQueryClient();

  const { data, isLoading: projectsLoading, isError: projectsError, refetch } = useQuery({
    queryKey: ['projects', page, limit ?? 12],
    queryFn: async () => {
      const { data } = await api.get<{ projects: Project[]; total: number; page: number; limit: number }>(
        '/projects',
        { params: { page, limit: limit ?? 12 } }
      );
      return data;
    },
    enabled: !prefsLoading && !prefsError && limit != null,
  });

  const createMutation = useMutation({
    mutationFn: (body: CreateForm) => api.post('/projects', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setPage(1);
      setDialogOpen(false);
      reset();
    },
  });

  const { register, handleSubmit, formState: { errors }, reset } = useForm<CreateForm>({
    resolver: zodResolver(createSchema),
  });

  const onSubmit = (data: CreateForm) => createMutation.mutate(data);

  if (prefsLoading || projectsLoading) {
    return (
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <Skeleton className="h-8 w-40" />
          <Skeleton className="h-10 w-32" />
        </div>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-32 rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  if (prefsError || projectsError) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <AlertCircle className="h-12 w-12 text-destructive mb-4" />
        <h2 className="text-lg font-semibold">Failed to load projects</h2>
        <p className="text-sm text-muted-foreground mb-4">Something went wrong. Please try again.</p>
        <Button onClick={() => refetch()}>Retry</Button>
      </div>
    );
  }

  const projects = data?.projects ?? [];
  const total = data?.total ?? 0;
  const effectiveLimit = limit ?? 12;
  const totalPages = Math.max(1, Math.ceil(total / effectiveLimit));
  const start = total === 0 ? 0 : (page - 1) * effectiveLimit + 1;
  const end = Math.min(total, page * effectiveLimit);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Projects</h1>
        <Button onClick={() => setDialogOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          New Project
        </Button>
      </div>

      {projects.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border py-20 text-center">
          <FolderOpen className="h-12 w-12 text-muted-foreground mb-4" />
          <h2 className="text-lg font-semibold">No projects yet</h2>
          <p className="text-sm text-muted-foreground mb-4">Create your first project to get started.</p>
          <Button onClick={() => setDialogOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Create Project
          </Button>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {projects.map((project) => (
            <Link
              key={project.id}
              to={`/projects/${project.id}`}
              className="group rounded-lg border border-border p-5 transition-colors hover:border-primary/50 hover:bg-accent/50"
            >
              <h3 className="font-semibold group-hover:text-primary transition-colors">{project.name}</h3>
              <p className="mt-1 text-sm text-muted-foreground line-clamp-2">
                {project.description || 'No description'}
              </p>
              <p className="mt-3 text-xs text-muted-foreground">
                Created {formatDate(project.created_at)}
              </p>
            </Link>
          ))}
        </div>
      )}

      {total > 0 && (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-muted-foreground">
            Showing <span className="text-foreground">{start}</span>–<span className="text-foreground">{end}</span> of{' '}
            <span className="text-foreground">{total}</span>
          </p>
          <div className="flex items-center gap-2">
            <select
              className="h-9 rounded-md border border-input bg-background px-2 text-sm"
              value={effectiveLimit}
              onChange={(e) => {
                updatePrefs.mutate({ projects_page_size: Number(e.target.value) });
                setPage(1);
              }}
              aria-label="Projects per page"
            >
              <option value={6}>6 / page</option>
              <option value={12}>12 / page</option>
              <option value={24}>24 / page</option>
            </select>
            <Button
              variant="outline"
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1}
            >
              Prev
            </Button>
            <div className="px-2 text-sm text-muted-foreground">
              Page <span className="text-foreground">{page}</span> / <span className="text-foreground">{totalPages}</span>
            </div>
            <Button
              variant="outline"
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
            >
              Next
            </Button>
          </div>
        </div>
      )}

      <Dialog open={dialogOpen} onClose={() => { setDialogOpen(false); reset(); }}>
        <DialogHeader>
          <DialogTitle>Create Project</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="project-name">Name</Label>
            <Input id="project-name" placeholder="My Project" {...register('name')} />
            {errors.name && <p className="text-sm text-destructive">{errors.name.message}</p>}
          </div>
          <div className="space-y-2">
            <Label htmlFor="project-desc">Description</Label>
            <Textarea id="project-desc" placeholder="Optional description" {...register('description')} />
          </div>
          {createMutation.isError && (
            <p className="text-sm text-destructive">Failed to create project. Please try again.</p>
          )}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => { setDialogOpen(false); reset(); }}>
              Cancel
            </Button>
            <Button type="submit" disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  );
}
