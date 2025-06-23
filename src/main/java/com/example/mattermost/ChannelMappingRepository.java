package com.example.mattermost;

import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface ChannelMappingRepository extends org.springframework.data.jpa.repository.JpaRepository<ChannelMapping, Long> {
    Optional<ChannelMapping> findByChannelId(String channelId);

    List<ChannelMapping> findByOwnerUserId(String ownerUserId);

    List<ChannelMapping> findByOtherUserId(String otherUserId);

    Optional<ChannelMapping> findByOwnerUserIdAndOtherUserId(String ownerUserId, String otherUserId);
}