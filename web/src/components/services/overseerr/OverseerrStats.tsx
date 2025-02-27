/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useState } from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { OverseerrMessage } from "./OverseerrMessage";
import { OverseerrMediaRequest } from "../../../types/service";
import { OverseerrRequestModal } from "./OverseerrRequestModal";
import { CheckCircleIcon, XCircleIcon } from "@heroicons/react/24/outline";
import { api } from "../../../utils/api";
import { toast } from "react-hot-toast";
import Toast from "../../Toast";

interface OverseerrStatsProps {
  instanceId: string;
}

export const OverseerrStats: React.FC<OverseerrStatsProps> = ({
  instanceId,
}) => {
  const { services, refreshService } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);
  const [localRequests, setLocalRequests] = useState<OverseerrMediaRequest[]>(
    []
  );
  const requests =
    localRequests.length > 0
      ? localRequests
      : service?.stats?.overseerr?.requests || [];
  const pendingRequests = requests.filter((req) => req.status === 1);
  const pendingCount = pendingRequests.length;
  const isLoading = !service || service.status === "loading";
  const error = service?.status === "error" ? service.message : null;

  const [selectedRequest, setSelectedRequest] =
    useState<OverseerrMediaRequest | null>(null);
  const [modalAction, setModalAction] = useState<"approve" | "reject" | null>(
    null
  );

  const handleAction = async (
    request: OverseerrMediaRequest,
    action: "approve" | "reject"
  ) => {
    setSelectedRequest(request);
    setModalAction(action);
  };

  const handleConfirmAction = async () => {
    if (!selectedRequest || !modalAction) return;

    try {
      const status = modalAction === "approve" ? 2 : 3; // 2 for approved, 3 for declined
      await api.post(
        `/api/services/${instanceId}/overseerr/request/${selectedRequest.id}/${status}`
      );

      // Update local state immediately
      const updatedRequests = requests.map((req) =>
        req.id === selectedRequest.id ? { ...req, status } : req
      );
      setLocalRequests(updatedRequests);

      // Show success toast
      toast.custom((t) => (
        <Toast
          type="success"
          body={`Successfully ${modalAction}d request for ${
            selectedRequest.media.title || "media"
          }`}
          t={t}
        />
      ));

      // Refresh service data in background
      refreshService(instanceId, "stats");
      setSelectedRequest(null);
      setModalAction(null);
    } catch (error) {
      console.error("Failed to update request status:", error);
      toast.custom((t) => (
        <Toast
          type="error"
          body={`Failed to ${modalAction} request: ${error}`}
          t={t}
        />
      ));
    }
  };

  if (isLoading) {
    return <p className="text-xs text-gray-500">Loading requests...</p>;
  }

  if (error) {
    return <p className="text-xs text-gray-500">Error: {error}</p>;
  }

  // Combine service message with health message if available
  const message = service.health?.message
    ? service.message
      ? `${service.message}\n${service.health.message}`
      : service.health.message
    : service.message;

  const getStatusLabel = (status: number) => {
    switch (status) {
      case 1:
        return "Pending";
      case 2:
        return "Approved";
      case 3:
        return "Declined";
      default:
        return "Unknown";
    }
  };

  const getStatusColor = (status: number) => {
    switch (status) {
      case 1:
        return "text-yellow-500";
      case 2:
        return "text-green-500";
      case 3:
        return "text-red-500";
      default:
        return "text-gray-500";
    }
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const getUserDisplayName = (
    requestedBy: OverseerrMediaRequest["requestedBy"]
  ) => {
    if (!requestedBy) return "Unknown User";
    return (
      requestedBy.username ||
      requestedBy.plexUsername ||
      requestedBy.email ||
      "Unknown User"
    );
  };

  const getMediaType = (request: OverseerrMediaRequest) => {
    return request.media.tvdbId ? "TV" : "Movie";
  };

  const getMediaTitle = (request: OverseerrMediaRequest) => {
    if (request.media.title) {
      return request.media.title;
    }
    return request.media.tvdbId
      ? `TV Show (TVDB: ${request.media.tvdbId})`
      : `Movie (TMDB: ${request.media.tmdbId})`;
  };

  const RequestActions = ({ request }: { request: OverseerrMediaRequest }) => (
    <div className="flex items-center gap-1">
      <button
        onClick={() => handleAction(request, "approve")}
        className="p-1 rounded-full hover:bg-gray-700/50 text-green-500 transition-colors"
        title="Approve request"
      >
        <CheckCircleIcon className="h-5 w-5" />
      </button>
      <button
        onClick={() => handleAction(request, "reject")}
        className="p-1 rounded-full hover:bg-gray-700/50 text-red-500 transition-colors"
        title="Reject request"
      >
        <XCircleIcon className="h-5 w-5" />
      </button>
    </div>
  );

  const RequestItem = ({ request }: { request: OverseerrMediaRequest }) => (
    <div className="border-b border-gray-800 last:border-0 pb-2 last:pb-0 space-y-1">
      <div className="flex justify-between items-center">
        <div className="font-medium">{getMediaTitle(request)}</div>
        <div className="flex items-center gap-2 ml-2 shrink-0">
          {request.status === 1 && <RequestActions request={request} />}
          <span
            className={`${getStatusColor(request.status)} text-xs font-medium`}
          >
            {getStatusLabel(request.status)}
          </span>
        </div>
      </div>
      <div className="text-gray-500 flex items-center gap-2 flex-wrap pt-1">
        <span>Requested by: {getUserDisplayName(request.requestedBy)}</span>
        <span>•</span>
        <span>{formatDate(request.createdAt)}</span>
        <span>•</span>
        <span className="text-[10px] font-medium text-gray-500">
          {getMediaType(request)}
        </span>
      </div>
    </div>
  );

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      <OverseerrMessage status={service.status} message={message} />

      {/* Pending Requests */}
      {pendingCount > 0 && (
        <div>
          <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
            Pending Requests:
          </div>
          <div className="text-xs rounded-md text-gray-700 dark:text-gray-400 bg-gray-850/95 p-4 overflow-hidden space-y-2">
            {pendingRequests.map((request) => (
              <RequestItem key={request.id} request={request} />
            ))}
          </div>
        </div>
      )}

      {/* Recent Requests List */}
      {requests.length > 0 && (
        <div>
          <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
            Recent Requests:
          </div>
          <div className="text-xs rounded-md text-gray-700 dark:text-gray-400 bg-gray-850/95 p-4 space-y-2">
            {requests
              .filter((request) => request.status !== 1)
              .slice(0, 5)
              .map((request) => (
                <RequestItem key={request.id} request={request} />
              ))}
          </div>
        </div>
      )}

      {/* Confirmation Modal */}
      {selectedRequest && modalAction && (
        <OverseerrRequestModal
          isOpen={true}
          onClose={() => {
            setSelectedRequest(null);
            setModalAction(null);
          }}
          request={selectedRequest}
          onConfirm={handleConfirmAction}
          action={modalAction}
        />
      )}
    </div>
  );
};
