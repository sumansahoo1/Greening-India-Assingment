import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import api from '@/lib/api';
import { useAuth } from '@/hooks/useAuth';

export type Preferences = {
  projects_page_size: number;
  tasks_page_size: number;
};

export function usePreferences() {
  const { user, token } = useAuth();
  return useQuery({
    queryKey: ['preferences', user?.id],
    queryFn: async () => {
      const { data } = await api.get<Preferences>('/me/preferences');
      return data;
    },
    staleTime: 5 * 60 * 1000,
    enabled: !!token,
  });
}

export function useUpdatePreferences() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (prefs: Partial<Preferences>) => {
      const { data } = await api.patch<Preferences>('/me/preferences', prefs);
      return data;
    },
    onSuccess: (data) => {
      queryClient.setQueryData(['preferences'], data);
    },
  });
}

