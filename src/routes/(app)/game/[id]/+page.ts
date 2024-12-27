import { error } from "@sveltejs/kit";
import type { PageLoad } from "./$types";

export const load = (async ({ params }) => {
  const { id } = params;

  if (!id) {
    error(400);
    return;
  }

  return {
    gameId: Number(id)
  };
}) satisfies PageLoad;
